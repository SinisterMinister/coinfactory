package coinfactory

import (
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/shopspring/decimal"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

// OrderRequest contains the information required to place an order through the API
type OrderRequest struct {
	Symbol   string
	Side     string
	Quantity decimal.Decimal
	Price    decimal.Decimal
}

// Order contains the state of the order
type Order struct {
	OrderRequest
	orderAck    binance.OrderResponseAckResponse
	orderStatus binance.OrderStatusResponse
	mux         *sync.Mutex
}

// GetStatus
func (o *Order) GetStatus() binance.OrderStatusResponse {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.orderStatus
}

type OrderLogger interface {
	LogOrder(order binance.OrderStatusResponse) error
}

type OrderManager interface {
	AttemptOrder(req OrderRequest) (*Order, error)
	AttemptTestOrder(req OrderRequest) error
	CancelOrder(order *Order) error
}

// Singleton implementation of OrderManager
type orderManager struct {
	openOrders    map[int]*Order
	lastSeenTrade map[string]int
	orderLogger   OrderLogger
}

var instance OrderManager
var once sync.Once

func getOrderManagerInstance() OrderManager {
	once.Do(func() {
		i := &orderManager{
			openOrders:    map[int]*Order{},
			lastSeenTrade: map[string]int{},
		}
		i.startOrderWatcher()
		instance = i
	})

	return instance
}

func (om *orderManager) AttemptOrder(order OrderRequest) (*Order, error) {
	req := binance.OrderRequest{
		Symbol:      order.Symbol,
		Side:        order.Side,
		Type:        "LIMIT",
		Quantity:    order.Quantity,
		Price:       order.Price,
		TimeInForce: "GTC",
	}
	log.WithFields(log.Fields{
		"Symbol":   order.Symbol,
		"Side":     order.Side,
		"Quantity": order.Quantity,
		"Price":    order.Price,
	}).Info("Placing order")

	symbol := binance.GetSymbol(order.Symbol)
	var err error

	if order.Side == "BUY" {
		err = localBalanceManagerInstance.freezeAmount(symbol.QuoteAsset, order.Price.Mul(order.Quantity))
	} else {
		err = localBalanceManagerInstance.freezeAmount(symbol.BaseAsset, order.Quantity)
	}

	if err != nil {
		return nil, err
	}

	res, err := binance.PlaceOrderGetAck(req)
	if err != nil {
		log.WithError(err).Debug("Order attempt failed!")
		return nil, err
	}
	om.openOrders[res.OrderID] = orderBuilder(order, res)

	return om.openOrders[res.OrderID], nil
}

func (om *orderManager) AttemptTestOrder(order OrderRequest) error {

	req := binance.OrderRequest{
		Symbol:      order.Symbol,
		Side:        order.Side,
		Type:        "LIMIT",
		Quantity:    order.Quantity,
		Price:       order.Price,
		TimeInForce: "GTC",
	}
	log.WithFields(log.Fields{
		"Symbol":   order.Symbol,
		"Side":     order.Side,
		"Quantity": order.Quantity,
		"Price":    order.Price,
	}).Info("Placing test order")

	err := binance.PlaceTestOrder(req)
	if err != nil {
		log.WithError(err).Debug("Order attempt failed!")
		return err
	}

	return nil
}

func (om *orderManager) CancelOrder(order *Order) error {
	cr := binance.OrderCancellationRequest{
		Symbol:  order.Symbol,
		OrderID: order.orderAck.OrderID,
	}

	_, err := binance.CancelOrder(cr)
	if err != nil {
		log.WithError(err).Debug("Order cancel attempt failed!")
		return err
	}

	return nil
}

func (om *orderManager) logOrder(order binance.OrderStatusResponse) {
	err := om.orderLogger.LogOrder(order)
	if err != nil {
		log.WithError(err).WithField("order", order).Error("Could not log order!")
	}
}

func orderBuilder(req OrderRequest, ack binance.OrderResponseAckResponse) *Order {
	return &Order{
		req,
		ack,
		binance.OrderStatusResponse{},
		&sync.Mutex{},
	}
}

func (om *orderManager) startOrderWatcher() {
	// Start a ticker for polling
	ticker := time.NewTicker(time.Duration(viper.GetInt("orderUpdateInterval")) * time.Second)

	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go om.watchOrders(ticker, interrupt)
}

func (om *orderManager) watchOrders(ticker *time.Ticker, interrupt chan os.Signal) {
	select {
	case <-ticker.C:
		log.Debug("Updating order statuses")
		for _, o := range om.openOrders {
			om.updateOrderStatus(o)
		}
		om.updateBalancesFromTrades()
	case <-interrupt:
		ticker.Stop()
		return
	}
}

// This needs to be rewritten using the trade stream service
func (om *orderManager) updateBalancesFromTrades() {
	for _, s := range fetchWatchedSymbols() {
		req := binance.TradeRequest{Symbol: s}
		if last, ok := om.lastSeenTrade[s]; ok {
			req.FromTradeID = last
		}

		trades, err := binance.GetTrades(req)
		if err != nil {
			log.WithError(err).Error("Error fetching trades for ", s)
			continue
		}

		if _, ok := om.lastSeenTrade[s]; !ok {
			lastTrade := trades[:len(trades)-1][0]
			req.FromTradeID = lastTrade.ID
			return
		}

		symbol := binance.GetSymbol(s)

		for _, t := range trades {
			// Update balance
			if t.IsBuyer {
				// Add purchased coin
				localBalanceManagerInstance.addFreeAmount(symbol.BaseAsset, t.Quantity)

				// Subtract used coin
				localBalanceManagerInstance.subtractLockedAmount(symbol.BaseAsset, t.Quantity.Mul(t.Price))

			} else {
				// Add sold coin
				localBalanceManagerInstance.subtractLockedAmount(symbol.BaseAsset, t.Quantity)

				// Subtract used coin
				localBalanceManagerInstance.addFreeAmount(symbol.BaseAsset, t.Quantity.Mul(t.Price))
			}

			// Subtract commission
			localBalanceManagerInstance.subtractFreeAmount(t.CommissionAsset, t.Commission)
		}
	}
}

func (om *orderManager) updateOrderStatus(order *Order) {
	res, err := binance.GetOrderStatus(binance.OrderStatusRequest{
		Symbol:  order.Symbol,
		OrderID: order.orderAck.OrderID,
	})
	if err != nil {
		log.WithError(err).WithField("order", order).Error("Could not update order status!")
		return
	}
	order.mux.Lock()
	switch res.Status {
	case "FILLED":
		fallthrough
	case "CANCELED":
		fallthrough
	case "REJECTED":
		fallthrough
	case "EXPIRED":
		delete(om.openOrders, order.orderAck.OrderID)
	}
	order.orderStatus = res
	order.mux.Unlock()
}
