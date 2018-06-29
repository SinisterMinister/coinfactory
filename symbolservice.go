package coinfactory

import (
	"errors"
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type SymbolService interface {
	GetSymbol(symbol string) (*Symbol, error)
}

type symbolService struct {
	symbols                 map[string]*Symbol
	tickerStreamInitialized chan bool
	tickerStreamDone        chan bool
}

var (
	symbolServiceOnce        sync.Once
	symbolServiceInstance    *symbolService
	symbolServiceInitialized = false
)

func (s *symbolService) GetSymbol(symbol string) (*Symbol, error) {
	ret, ok := s.symbols[symbol]
	if !ok {
		return &Symbol{}, errors.New("symbol does not exist")
	}

	return ret, nil
}

func (s *symbolService) initializeSymbols() {
	s.symbols = make(map[string]*Symbol)

	rawSymbols := binance.GetSymbols()
	for k, v := range rawSymbols {
		s.symbols[k] = &Symbol{v, binance.SymbolTickerData{}}
	}
}

func (s *symbolService) ReceiveData(data []binance.SymbolTickerData) {
	// Update symbols
	for _, ticker := range data {
		symbol, err := s.GetSymbol(ticker.Symbol)
		if err != nil {
			log.WithField("ticker", ticker).WithError(err).Error("SymbolService: could not update ticker")
			continue
		}
		symbol.Ticker = ticker
	}
	if !symbolServiceInitialized {
		s.tickerStreamInitialized <- true
	}
}

func (s *symbolService) startTickerStream() {
	s.tickerStreamInitialized = make(chan bool)
	s.tickerStreamDone = binance.GetAllMarketTickersStream(s)
	symbolServiceInitialized = <-s.tickerStreamInitialized
}

func (s *symbolService) start() {
	log.Info("Starting symbol service")
	defer log.Info("Symbol service started successfully")
	s.initializeSymbols()
	s.startTickerStream()

}

func (s *symbolService) stop() {
	log.Warn("Stopping symbol service")
	defer log.Warn("Symbol service stopped")
	// Kill the handler
	s.tickerStreamDone <- true
	symbolServiceInitialized = false
}

func GetSymbolService() SymbolService {
	symbolServiceOnce.Do(func() {
		if !symbolServiceInitialized {
			symbolServiceInstance = &symbolService{}
			symbolServiceInstance.start()
			symbolServiceInitialized = true
		}
	})

	return symbolServiceInstance
}
