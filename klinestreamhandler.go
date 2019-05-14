package coinfactory

import (
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
)

type KlineStreamManager interface {
	GetKlineStream(done <-chan bool) <-chan binance.KlineStreamData
}
type klineStreamManager struct {
	broadcastChannels map[string]chan binance.KlineStreamData
	channelMux        *sync.Mutex
}

func (h *klineStreamManager) GetKlineStream(done <-chan bool, symbol string, interval string) <-chan binance.KlineStreamData {
	// Setup the channel
	payloadChan := binance.GetKlineStream(done, symbol, interval)
	dataChan := make(chan binance.KlineStreamData)

	go func() {
		for {
			select {
			case <-done:
				return
			case p := <-payloadChan:
				dataChan <- p.KlineData
			}
		}
	}()

	return dataChan
}
