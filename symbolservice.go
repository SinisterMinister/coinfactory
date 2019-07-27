package coinfactory

import (
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
)

type SymbolService interface {
	GetSymbol(symbol string) *Symbol
}

type symbolService struct {
	symbolStopChan chan bool
}

var (
	symbolServiceOnce     sync.Once
	symbolServiceInstance *symbolService
)

func (s *symbolService) GetSymbol(symbol string) *Symbol {
	return newSymbol(binance.GetSymbol(symbol), s.symbolStopChan)
}

func (s *symbolService) start() {
	// NOOP
}

func (s *symbolService) stop() {
	// Close the ticker streams for the all the various symbols
	select {
	default:
		close(s.symbolStopChan)
	case <-s.symbolStopChan:
	}
}

func getSymbolService() *symbolService {
	symbolServiceOnce.Do(func() {
		symbolServiceInstance = &symbolService{
			symbolStopChan: make(chan bool),
		}
	})

	return symbolServiceInstance
}
