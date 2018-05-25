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
	orderAck          binance.OrderResponseAckResponse
	orderStatus       binance.OrderStatusResponse
	orderCreationTime time.Time
	mux               *sync.Mutex
}

// GetStatus
func (o *Order) GetStatus() binance.OrderStatusResponse {
	o.mux.Lock()
	defer o.mux.Unlock()
	return o.orderStatus
}

// GetAge returns the age of the order
func (o *Order) GetAge() time.Duration {
	o.mux.Lock()
	defer o.mux.Unlock()
	return time.Since(o.orderCreationTime)
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
	openOrdersMux *sync.Mutex
}

var instance OrderManager
var once sync.Once

func getOrderManagerInstance() OrderManager {
	once.Do(func() {
		i := &orderManager{
			openOrders:    map[int]*Order{},
			lastSeenTrade: map[string]int{},
			openOrdersMux: &sync.Mutex{},
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
	om.openOrdersMux.Lock()
	defer om.openOrdersMux.Unlock()
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
		time.Now(),
		&sync.Mutex{},
	}
}

func (om *orderManager) startOrderWatcher() {
	go func() {
		// Start a ticker for polling
		ticker := time.NewTicker(time.Duration(viper.GetInt("orderUpdateInterval")) * time.Second)

		// Intercept the interrupt signal and pass it along
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		for {
			select {
			case <-ticker.C:
				log.Info("Updating order statuses")
				om.openOrdersMux.Lock()
				cp := make(map[int]*Order)
				for k, v := range om.openOrders {
					cp[k] = v
				}
				om.openOrdersMux.Unlock()
				for _, o := range cp {
					om.updateOrderStatus(o)
				}
			case <-interrupt:
				ticker.Stop()
				log.Warn("Stopping order watcher")
				defer log.Warn("Order watcher stopped")
				return
			}
		}
	}()
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
		om.openOrdersMux.Lock()
		delete(om.openOrders, order.orderAck.OrderID)
		om.openOrdersMux.Unlock()
	}
	order.orderStatus = res
	order.mux.Unlock()
}
