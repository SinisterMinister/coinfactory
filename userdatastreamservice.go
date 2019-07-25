package coinfactory

import (
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type UserDataStreamService interface {
	GetAccountUpdateStream(stopChan <-chan bool) <-chan binance.AccountUpdatePayload
	GetOrderUpdateStream(stopChan <-chan bool) <-chan binance.OrderUpdatePayload
}

type userDataStreamService struct {
	connectionMutex           *sync.Mutex
	connectionHandlerDoneChan chan bool
	running                   bool
	runningMutex              *sync.RWMutex
	accountUpdateStreams      []chan binance.AccountUpdatePayload
	orderUpdateStreams        []chan binance.OrderUpdatePayload
	streamMutex               *sync.RWMutex
}

var (
	userDataStreamServiceInstance *userDataStreamService
	sdssOnce                      sync.Once
)

func getUserDataStreamService() *userDataStreamService {
	sdssOnce.Do(func() {
		userDataStreamServiceInstance = &userDataStreamService{
			&sync.Mutex{},
			make(chan bool),
			false,
			&sync.RWMutex{},
			[]chan binance.AccountUpdatePayload{},
			[]chan binance.OrderUpdatePayload{},
			&sync.RWMutex{},
		}
	})

	return userDataStreamServiceInstance
}

func (udss *userDataStreamService) GetAccountUpdateStream(stopChan <-chan bool) <-chan binance.AccountUpdatePayload {
	channel := make(chan binance.AccountUpdatePayload)
	udss.registerChannel(channel)

	// Remove registration when stop channel is closed
	go func(stopChan <-chan bool) {
		select {
		case <-stopChan:
			udss.deregisterChannel(channel)
		}
	}(stopChan)

	return channel
}

func (udss *userDataStreamService) GetOrderUpdateStream(stopChan <-chan bool) <-chan binance.OrderUpdatePayload {
	channel := make(chan binance.OrderUpdatePayload)
	udss.registerChannel(channel)

	// Remove registration when stop channel is closed
	go func(stopChan <-chan bool) {
		select {
		case <-stopChan:
			udss.deregisterChannel(channel)
		}
	}(stopChan)

	return channel
}

func (udss *userDataStreamService) registerChannel(channel interface{}) {
	udss.streamMutex.Lock()
	defer udss.streamMutex.Unlock()

	switch stream := channel.(type) {
	case chan binance.AccountUpdatePayload:
		udss.accountUpdateStreams = append(udss.accountUpdateStreams, stream)
	case chan binance.OrderUpdatePayload:
		udss.orderUpdateStreams = append(udss.orderUpdateStreams, stream)
	}
}

func (udss *userDataStreamService) deregisterChannel(channel interface{}) {
	udss.streamMutex.Lock()
	defer udss.streamMutex.Unlock()

	switch stream := channel.(type) {
	case chan binance.AccountUpdatePayload:
		filtered := udss.accountUpdateStreams[:0]
		for _, c := range udss.accountUpdateStreams {
			if c != stream {
				filtered = append(filtered, c)
			} else {
				// Close the channel gracefully
				select {
				case <-c:
				default:
					close(c)
				}
			}
		}

		// Clean up references
		for i := len(filtered); i < len(udss.accountUpdateStreams); i++ {
			udss.accountUpdateStreams[i] = nil
		}

		udss.accountUpdateStreams = filtered
	case chan binance.OrderUpdatePayload:
		filtered := udss.orderUpdateStreams[:0]
		for _, c := range udss.orderUpdateStreams {
			if c != stream {
				filtered = append(filtered, c)
			} else {
				// Close the channel gracefully
				select {
				case <-c:
				default:
					close(c)
				}
			}
		}

		// Clean up references
		for i := len(filtered); i < len(udss.orderUpdateStreams); i++ {
			udss.orderUpdateStreams[i] = nil
		}

		udss.orderUpdateStreams = filtered
	}
}

func (udss *userDataStreamService) start() {
	udss.runningMutex.Lock()
	udss.running = true
	udss.runningMutex.Unlock()
	udss.refreshConnection()
}

func (udss *userDataStreamService) stop() {
	udss.connectionMutex.Lock()
	close(udss.connectionHandlerDoneChan)
	udss.connectionMutex.Unlock()

	udss.runningMutex.Lock()
	udss.running = false
	udss.runningMutex.Unlock()
}

func (udss *userDataStreamService) refreshConnection() {
	// Start a new connection
	newDoneChan := make(chan bool)
	go udss.connectionHandler(newDoneChan)

	udss.connectionMutex.Lock()

	// Shutdown the old connection if needed
	select {
	case <-udss.connectionHandlerDoneChan:
	default:
		close(udss.connectionHandlerDoneChan)
	}

	// Store the done channel
	udss.connectionHandlerDoneChan = newDoneChan
	udss.connectionMutex.Unlock()
}

func (udss *userDataStreamService) connectionHandler(stopChan <-chan bool) {
	userDataStream := binance.GetUserDataStream(stopChan)

	// Watch the stream
	for {
		// Bail on stopChan
		select {
		case <-stopChan:
			return
		default:
		}

		// Handle the data
		select {
		case <-stopChan:
			return
		case data := <-userDataStream:
			udss.broadcastDataToChannels(data)
		}
	}
}

func (udss *userDataStreamService) broadcastDataToChannels(data binance.UserDataPayload) {
	if data.AccountUpdatePayload.EventTime > 0 {
		for _, channel := range udss.accountUpdateStreams {
			select {
			case channel <- data.AccountUpdatePayload:
			default:
				log.Warn("Skipping sending data: blocked account update channel")
			}
		}
	}

	if data.OrderUpdatePayload.EventTime > 0 {
		for _, channel := range udss.orderUpdateStreams {
			select {
			case channel <- data.OrderUpdatePayload:
			default:
				log.Warn("Skipping sending data: blocked order update channel")
			}
		}
	}
}
