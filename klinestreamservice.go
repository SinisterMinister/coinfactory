package coinfactory

import (
	"strings"
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type KlineStreamService interface {
	GetKlineStream(done <-chan bool, symbol string, interval string) <-chan binance.KlineStreamData
}
type klineStreamService struct {
	connectionMutex           *sync.Mutex
	connectionHandlerDoneChan chan bool
	streams                   map[string][]chan binance.KlineStreamData
	streamsMutex              *sync.RWMutex
	running                   bool
	runningMutex              *sync.RWMutex
}

var (
	klineStreamServiceInstance *klineStreamService
	kssOnce                    sync.Once
)

func getKlineStreamService() *klineStreamService {
	kssOnce.Do(func() {
		klineStreamServiceInstance = &klineStreamService{
			&sync.Mutex{},
			make(chan bool),
			make(map[string][]chan binance.KlineStreamData),
			&sync.RWMutex{},
			false,
			&sync.RWMutex{},
		}
	})
	return klineStreamServiceInstance
}

func (kss *klineStreamService) getSymbolIntervals() []binance.KlineSymbolInterval {
	sis := []binance.KlineSymbolInterval{}

	kss.streamsMutex.RLock()
	for siRaw := range kss.streams {
		si := strings.Split(siRaw, "_")
		sis = append(sis, binance.KlineSymbolInterval{
			Symbol:   si[0],
			Interval: si[1],
		})
	}
	kss.streamsMutex.RUnlock()

	return sis
}

func (kss *klineStreamService) connectionHandler(stopChan <-chan bool) {
	// Start the stream
	klineStream := binance.GetCombinedKlineStream(stopChan, kss.getSymbolIntervals())

	// Watch the stream
	for {
		select {
		case <-stopChan:
			return
		case data := <-klineStream:
			for _, klineData := range data {
				symInt := binance.KlineSymbolInterval{Symbol: klineData.Symbol, Interval: klineData.KlineData.Interval}
				kss.broadcastDataToChannels(symInt.GetStreamName(), klineData)
			}
		}
	}
}

func (kss *klineStreamService) GetKlineStream(stopChan <-chan bool, symbol string, interval string) <-chan binance.KlineStreamData {
	channel := make(chan binance.KlineStreamData)
	symInt := binance.KlineSymbolInterval{
		Symbol:   symbol,
		Interval: interval,
	}
	kss.registerChannel(symInt.GetStreamName(), channel)

	// Remove registration when stop channel is closed
	go func(stopChan <-chan bool, streamName string) {
		select {
		case <-stopChan:
			kss.deregisterChannel(streamName, channel)
		}
	}(stopChan, symInt.GetStreamName())

	return channel
}

func (kss *klineStreamService) registerChannel(streamName string, dataChan chan binance.KlineStreamData) {
	kss.streamsMutex.RLock()
	slice, ok := kss.streams[streamName]
	kss.streamsMutex.RUnlock()

	// Setup the slice if it doesn't exist
	if !ok {
		slice = []chan binance.KlineStreamData{}
		kss.streamsMutex.Lock()
		kss.streams[streamName] = slice
		kss.streamsMutex.Unlock()
	}

	kss.streamsMutex.Lock()
	kss.streams[streamName] = append(kss.streams[streamName], dataChan)
	kss.streamsMutex.Unlock()

	// Refresh the connection if we're running
	kss.runningMutex.RLock()
	if kss.running {
		kss.refreshConnection()
	}
	kss.runningMutex.RUnlock()
}

func (kss *klineStreamService) deregisterChannel(streamName string, dataChan chan binance.KlineStreamData) {
	// Lock the tickers
	kss.streamsMutex.Lock()
	defer kss.streamsMutex.Unlock()

	// Bail out if there's no channels there already
	if _, ok := kss.streams[streamName]; !ok {
		return
	}

	filtered := kss.streams[streamName][:0]
	for _, c := range kss.streams[streamName] {
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
	for i := len(filtered); i < len(kss.streams[streamName]); i++ {
		kss.streams[streamName][i] = nil
	}

	kss.streams[streamName] = filtered
}

func (kss *klineStreamService) refreshConnection() {
	// Lock the mutex
	kss.streamsMutex.RLock()

	// Get the stream names to watch
	streamNames := make([]string, len(kss.streams))
	i := 0
	for s := range kss.streams {
		streamNames[i] = s
		i++
	}
	kss.streamsMutex.RUnlock()

	if len(streamNames) > 0 {
		// Start a new connection
		newDoneChan := make(chan bool)
		go kss.connectionHandler(newDoneChan)

		kss.connectionMutex.Lock()

		// Shutdown the old connection if needed
		select {
		case <-kss.connectionHandlerDoneChan:
		default:
			close(kss.connectionHandlerDoneChan)
		}

		// Store the done channel
		kss.connectionHandlerDoneChan = newDoneChan
		kss.connectionMutex.Unlock()
	}

}

func (kss *klineStreamService) broadcastDataToChannels(streamName string, data binance.KlineStreamPayload) {
	// Lock the tickers
	kss.streamsMutex.Lock()
	defer kss.streamsMutex.Unlock()

	// Bail out if there's no channels there already
	if _, ok := kss.streams[streamName]; !ok {
		log.Warn("Skipping sending data: no channels to send to")
		return
	}

	// Fan out the data to all the channels
	for _, channel := range kss.streams[streamName] {
		select {
		case channel <- data.KlineData:
		default:
			log.Warn("Skipping sending data: blocked ticker channel")
		}
	}
}

func (kss *klineStreamService) start() {
	kss.runningMutex.Lock()
	kss.running = true
	kss.runningMutex.Unlock()
	kss.refreshConnection()
}

func (kss *klineStreamService) stop() {
	kss.connectionMutex.Lock()
	close(kss.connectionHandlerDoneChan)
	kss.connectionMutex.Unlock()

	kss.runningMutex.Lock()
	kss.running = false
	kss.runningMutex.Unlock()
}
