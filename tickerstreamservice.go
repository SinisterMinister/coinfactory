package coinfactory

import (
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type TickerStreamService interface {
	GetTickerStream(stopChan <-chan bool, symbol string) <-chan binance.SymbolTickerData
}

type tickerStreamService struct {
	connectionMutex           *sync.Mutex
	connectionHandlerDoneChan chan bool
	tickers                   map[string][]chan binance.SymbolTickerData
	tickersMutex              *sync.RWMutex
	running                   bool
	runningMutex              *sync.RWMutex
}

var (
	tickerStreamServiceInstance *tickerStreamService
	tssOnce                     sync.Once
)

func getTickerStreamService() *tickerStreamService {
	once.Do(func() {
		tickerStreamServiceInstance = &tickerStreamService{
			&sync.Mutex{},
			make(chan bool),
			make(map[string][]chan binance.SymbolTickerData),
			&sync.RWMutex{},
			false,
			&sync.RWMutex{},
		}
	})
	return tickerStreamServiceInstance
}

func (tss *tickerStreamService) GetTickerStream(stopChan <-chan bool, symbol string) <-chan binance.SymbolTickerData {
	channel := make(chan binance.SymbolTickerData)
	tss.registerChannel(symbol, channel)

	// Remove registration when stop channel is closed
	go func(stopChan <-chan bool) {
		select {
		case <-stopChan:
			tss.deregisterChannel(symbol, channel)
		}
	}(stopChan)

	return channel
}

func (tss *tickerStreamService) registerChannel(symbol string, dataChan chan binance.SymbolTickerData) {
	tss.tickersMutex.RLock()
	slice, ok := tss.tickers[symbol]
	tss.tickersMutex.RUnlock()

	// Setup the slice if it doesn't exist
	if !ok {
		slice = []chan binance.SymbolTickerData{}
		tss.tickersMutex.Lock()
		tss.tickers[symbol] = slice
		tss.tickersMutex.Unlock()
	}

	// Refresh the connection if we're running
	tss.runningMutex.RLock()
	if tss.running {
		tss.refreshConnection()
	}
	tss.runningMutex.RUnlock()

	tss.tickersMutex.Lock()
	tss.tickers[symbol] = append(tss.tickers[symbol], dataChan)
	tss.tickersMutex.Unlock()
}

func (tss *tickerStreamService) deregisterChannel(symbol string, dataChan chan binance.SymbolTickerData) {
	// Lock the tickers
	tss.tickersMutex.Lock()
	defer tss.tickersMutex.Unlock()

	// Bail out if there's no channels there already
	if _, ok := tss.tickers[symbol]; !ok {
		return
	}

	filtered := tss.tickers[symbol][:0]
	for _, c := range tss.tickers[symbol] {
		if c != dataChan {
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
	for i := len(filtered); i < len(tss.tickers[symbol]); i++ {
		tss.tickers[symbol][i] = nil
	}

	tss.tickers[symbol] = filtered
}

func (tss *tickerStreamService) refreshConnection() {
	// Lock the mutex
	tss.tickersMutex.RLock()

	// Get the symbols to watch
	symbols := make([]string, len(tss.tickers))
	i := 0
	for s := range tss.tickers {
		symbols[i] = s
		i++
	}
	tss.tickersMutex.RUnlock()

	if len(symbols) > 0 {
		// Start a new connection
		newDoneChan := make(chan bool)
		go tss.connectionHandler(newDoneChan, symbols)

		tss.connectionMutex.Lock()

		// Shutdown the old connection if needed
		select {
		case <-tss.connectionHandlerDoneChan:
		default:
			close(tss.connectionHandlerDoneChan)
		}

		// Store the done channel
		tss.connectionHandlerDoneChan = newDoneChan
		tss.connectionMutex.Unlock()
	}

}

func (tss *tickerStreamService) connectionHandler(stopChan <-chan bool, symbols []string) {
	// Start the stream
	tickerStream := binance.GetCombinedTickerStream(stopChan, symbols)

	// Watch the stream
	for {
		select {
		case <-stopChan:
			return
		case data := <-tickerStream:
			for _, tickerData := range data {
				tss.broadcastDataToChannels(tickerData.Symbol, tickerData)
			}
		}
	}
}

func (tss *tickerStreamService) broadcastDataToChannels(symbol string, data binance.SymbolTickerData) {
	// Lock the tickers
	tss.tickersMutex.Lock()
	defer tss.tickersMutex.Unlock()

	// Bail out if there's no channels there already
	if _, ok := tss.tickers[symbol]; !ok {
		log.Warn("Skipping sending data: no channels to send to")
		return
	}

	// Fan out the data to all the channels
	for _, channel := range tss.tickers[symbol] {
		select {
		case channel <- data:
		default:
			log.Warn("Skipping sending data: blocked ticker channel")
		}
	}
}

func (tss *tickerStreamService) start() {
	tss.runningMutex.Lock()
	tss.running = true
	tss.runningMutex.Unlock()
	tss.refreshConnection()
}

func (tss *tickerStreamService) stop() {
	tss.connectionMutex.Lock()
	close(tss.connectionHandlerDoneChan)
	tss.connectionMutex.Unlock()

	tss.runningMutex.Lock()
	tss.running = false
	tss.runningMutex.Unlock()
}
