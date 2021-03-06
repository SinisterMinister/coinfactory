package coinfactory

import (
	"sync"
	"time"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

// Order contains the state of the order
type Order struct {
	OrderRequest
	orderStatus       binance.OrderStatusResponse
	orderCreationTime time.Time
	orderID           int
	orderMutex        *sync.Mutex
	doneChan          chan int
	stopChan          <-chan bool
	updateChannels    []chan bool
}

type orderBuilder struct {
	request      OrderRequest
	status       binance.OrderStatusResponse
	creationTime time.Time
	id           int
	stopChan     <-chan bool
}

func (ob *orderBuilder) withStatus(status binance.OrderStatusResponse) *orderBuilder {
	ob.status = status
	return ob
}

func (ob *orderBuilder) withCreationTime(creationTime time.Time) *orderBuilder {
	ob.creationTime = creationTime
	return ob
}

func (ob *orderBuilder) withID(id int) *orderBuilder {
	ob.id = id
	return ob
}

func (ob *orderBuilder) build() *Order {
	order := &Order{
		ob.request,
		ob.status,
		ob.creationTime,
		ob.id,
		&sync.Mutex{},
		make(chan int),
		ob.stopChan,
		[]chan bool{},
	}

	go order.orderStatusHandler(order.stopChan)

	return order
}

func newOrderBuilder(request OrderRequest, stopChan <-chan bool) *orderBuilder {
	return &orderBuilder{
		request:  request,
		stopChan: stopChan,
	}
}

// GetStatus
func (o *Order) GetStatus() binance.OrderStatusResponse {
	o.orderMutex.Lock()
	defer o.orderMutex.Unlock()
	return o.orderStatus
}

// GetAge returns the age of the order
func (o *Order) GetAge() time.Duration {
	o.orderMutex.Lock()
	defer o.orderMutex.Unlock()
	return time.Since(o.orderCreationTime)
}

func (o *Order) GetCreationTime() time.Time {
	o.orderMutex.Lock()
	defer o.orderMutex.Unlock()
	return o.orderCreationTime
}

func (o *Order) GetDoneChan() <-chan int {
	return o.doneChan
}

func (o *Order) GetUpdateChan() <-chan bool {

	ch := make(chan bool)

	// Add the channel to registry
	o.orderMutex.Lock()
	o.updateChannels = append(o.updateChannels, ch)
	o.orderMutex.Unlock()

	return ch
}

func (order *Order) orderStatusHandler(stopChan <-chan bool) {
	orderStream := getUserDataStreamService().GetOrderUpdateStream(stopChan)

	for {
		// Bail if stopchan is closed
		select {
		case <-stopChan:
			return
		default:
		}

		select {
		case <-stopChan:
			return
		case data := <-orderStream:
			if data.OrderID == order.orderID {
				log.Info("Updating order status")
				order.orderMutex.Lock()
				switch data.CurrentOrderStatus {
				case "FILLED":
					fallthrough
				case "CANCELED":
					fallthrough
				case "REJECTED":
					fallthrough
				case "EXPIRED":
					select {
					default:
						close(order.doneChan)
					case <-order.doneChan:
					}
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

				// Let all the channels know
				for _, ch := range order.updateChannels {
					select {
					case ch <- true:
						// Let the channel know that the order was updated
					default:
						// Skip blocked channel
						log.Warn("skipping blocked order status channel")
					}
				}
				order.orderMutex.Unlock()
			}
		}
	}
}
