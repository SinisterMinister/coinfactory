package coinfactory

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

// OrderRequest contains the information required to place an order through the API
type OrderRequest struct {
	Symbol   string          `json:"symbol"`
	Side     string          `json:"side"`
	Quantity decimal.Decimal `json:"qty"`
	Price    decimal.Decimal `json:"price"`
}

type OrderService interface {
	AttemptOrder(req OrderRequest) (*Order, error)
	AttemptTestOrder(req OrderRequest) error
	CancelOrder(order *Order) error
	GetOpenOrders(symbol string) ([]*Order, error)
	GetOrder(symbol string, orderId int) (*Order, error)
}

type orderService struct {
	orderStopChan  chan bool
	ordersThisTick int
	tickMux        *sync.Mutex
	tickWaitChan   chan bool
}

var (
	osOnce               sync.Once
	orderServiceInstance *orderService
)

func getOrderService() *orderService {
	osOnce.Do(func() {
		orderServiceInstance = &orderService{
			orderStopChan: make(chan bool),
			tickWaitChan:  make(chan bool),
			tickMux:       &sync.Mutex{},
		}

		orderServiceInstance.startTicker()
	})

	return orderServiceInstance
}

func (os *orderService) AttemptOrder(order OrderRequest) (*Order, error) {
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

	// Throttle requests to 10TPS
	if os.getTickOrderCount() >= 10 {
		<-os.tickWaitChan
	}
	os.incrementTickOrderCount()

	res, err := binance.PlaceOrderGetAck(req)
	if err != nil {
		log.WithError(err).Debug("Order attempt failed!")
		return nil, err
	}

	newOrder := newOrderBuilder(order, os.orderStopChan).withID(res.OrderID).build()

	return newOrder, nil
}

func (os *orderService) incrementTickOrderCount() {
	os.tickMux.Lock()
	defer os.tickMux.Unlock()
	os.ordersThisTick++
}

func (os *orderService) getTickOrderCount() int {
	os.tickMux.Lock()
	defer os.tickMux.Unlock()
	return os.ordersThisTick
}

func (os *orderService) nextTick() {
	os.tickMux.Lock()
	defer os.tickMux.Unlock()
	os.ordersThisTick = 0
	close(os.tickWaitChan)
	os.tickWaitChan = make(chan bool)
}

func (os *orderService) startTicker() {
	os.nextTick()
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			<-ticker.C
			os.nextTick()
		}
	}()
}

func (os *orderService) AttemptTestOrder(order OrderRequest) error {

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

func (os *orderService) CancelOrder(order *Order) error {
	cr := binance.OrderCancellationRequest{
		Symbol:  order.Symbol,
		OrderID: order.orderID,
	}

	_, err := binance.CancelOrder(cr)
	if err != nil {
		log.WithError(err).Debug("Order cancel attempt failed!")
		return err
	}

	return nil
}

func (os *orderService) GetOpenOrders(symbol string) (orders []*Order, err error) {
	rawOrders, err := binance.GetOpenOrders(binance.OpenOrdersRequest{Symbol: symbol})
	if err != nil {
		log.WithError(err).Error("could not get open orders")
		return orders, err
	}

	orders = []*Order{}

	for _, v := range rawOrders {
		ct := time.Unix(0, int64(v.Timestamp*1e6))
		order := newOrderBuilder(OrderRequest{
			Side:     v.Side,
			Symbol:   v.Symbol,
			Quantity: v.OriginalQuantity,
			Price:    v.Price,
		}, os.orderStopChan).withStatus(v).withCreationTime(ct).withID(v.OrderID).build()
		// Add to return
		orders = append(orders, order)
	}

	return orders, err
}

func (os *orderService) GetOrder(symbol string, orderID int) (order *Order, err error) {
	rawOrder, err := binance.GetOrderStatus(binance.OrderStatusRequest{Symbol: symbol, OrderID: orderID})
	if err != nil {
		log.WithError(err).Error("could not get open orders")
		return order, err
	}

	ct := time.Unix(0, int64(rawOrder.Timestamp*1e6))
	order = newOrderBuilder(OrderRequest{
		Side:     rawOrder.Side,
		Symbol:   rawOrder.Symbol,
		Quantity: rawOrder.OriginalQuantity,
		Price:    rawOrder.Price,
	}, os.orderStopChan).withStatus(rawOrder).withCreationTime(ct).withID(rawOrder.OrderID).build()

	return order, err
}
