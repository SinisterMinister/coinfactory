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

// Order contains the state of the order
type Order struct {
	OrderRequest
	orderStatus       binance.OrderStatusResponse
	orderCreationTime time.Time
	orderID           int
	mux               *sync.Mutex
	doneChan          chan int
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

func (o *Order) GetDoneChan() <-chan int {
	return o.doneChan
}

type OrderLogger interface {
	LogOrder(order binance.OrderStatusResponse) error
}

type OrderStreamProcessor struct {
	om *orderManager
}

func (processor *OrderStreamProcessor) ProcessUserData(data binance.UserDataPayload) {
	payload := data.OrderUpdatePayload
	if payload.EventTime != 0 {
		log.WithField("data", data.OrderUpdatePayload).Debug("Order data received")
		processor.om.updateOrderStatusFromStreamProcessor(data.OrderUpdatePayload)
	}
}

type OrderManager interface {
	AttemptOrder(req OrderRequest) (*Order, error)
	AttemptTestOrder(req OrderRequest) error
	CancelOrder(order *Order) error
	GetOpenOrders(symbol *Symbol) ([]*Order, error)
	GetOrderDataStreamProcessor() *OrderStreamProcessor
	UpdateOrderStatus(order *Order) error
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

		i.startOrderStreamWatcher()
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
		OrderID: order.orderID,
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
		binance.OrderStatusResponse{},
		time.Now(),
		ack.OrderID,
		&sync.Mutex{},
		make(chan int),
	}
}

func (om *orderManager) UpdateOrderStatus(order *Order) error {
	status, err := binance.GetOrderStatus(binance.OrderStatusRequest{Symbol: order.Symbol, OrderID: order.orderID})
	if err != nil {
		if e, ok := err.(binance.ResponseError); ok {
			body, _ := e.ResponseBodyString()
			log.WithError(err).WithField("res", body).Error("could not update order status")
		} else {
			log.WithError(err).Error("could not update order status")
		}
		return err
	}

	order.orderStatus = status
	switch status.Status {
	case "FILLED":
		fallthrough
	case "CANCELED":
		fallthrough
	case "REJECTED":
		fallthrough
	case "EXPIRED":
		select {
		case <-order.doneChan:
		default:
			close(order.doneChan)
		}
	default:
	}

	return nil
}

func (om *orderManager) updateOrderStatusFromStreamProcessor(data binance.OrderUpdatePayload) {
	om.openOrdersMux.Lock()
	order, ok := om.openOrders[data.OrderID]
	om.openOrdersMux.Unlock()
	if ok {
		log.Info("Updating order status")
		order.mux.Lock()
		switch data.CurrentOrderStatus {
		case "FILLED":
			fallthrough
		case "CANCELED":
			fallthrough
		case "REJECTED":
			fallthrough
		case "EXPIRED":
			om.openOrdersMux.Lock()
			delete(om.openOrders, data.OrderID)
			select {
			default:
				close(order.doneChan)
			case <-order.doneChan:
			}
			om.openOrdersMux.Unlock()
		}
		// Build an order status
		status := binance.OrderStatusResponse{
			order.Symbol,
			order.orderID,
			data.ClientOrderID,
			order.Price,
			data.OrderQuantity,
			data.FilledQuantity,
			data.CurrentOrderStatus,
			data.TimeInForce,
			data.OrderType,
			data.Side,
			data.StopPrice.String(),
			data.IcebergQuantity,
			data.OrderCreationTime,
			data.EventTime,
			data.IsWorking,
		}
		order.orderStatus = status
		order.mux.Unlock()
	}

}

func (manager *orderManager) GetOpenOrders(symbol *Symbol) (orders []*Order, err error) {
	rawOrders, err := binance.GetOpenOrders(binance.OpenOrdersRequest{Symbol: symbol.Symbol})
	if err != nil {
		log.WithError(err).Error("could not get open orders")
		return orders, err
	}

	orders = []*Order{}

	for _, v := range rawOrders {
		order := &Order{
			OrderRequest{
				Side:     v.Side,
				Symbol:   v.Symbol,
				Quantity: v.OriginalQuantity,
				Price:    v.Price,
			},
			v,
			time.Unix(0, int64(v.Timestamp*1e6)),
			v.OrderID,
			&sync.Mutex{},
			make(chan int),
		}
		// Add to return
		orders = append(orders, order)

		// Add to open orders list
		manager.openOrders[order.orderID] = order
	}

	return orders, err
}

func (om *orderManager) startOrderStreamWatcher() {
	getUserDataStreamHandlerInstance().registerProcessor("coinfactory.orderstreamprocessor", om.GetOrderDataStreamProcessor())
}

func (manager *orderManager) GetOrderDataStreamProcessor() *OrderStreamProcessor {
	return &OrderStreamProcessor{manager}
}
