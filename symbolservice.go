package coinfactory

import (
	"errors"
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
)

type SymbolService interface {
	GetSymbol(symbol string) (*Symbol, error)
}

type symbolService struct {
	symbols map[string]*Symbol
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

func (s *symbolService) init() {
	s.symbols = make(map[string]*Symbol)
	rawSymbols := binance.GetSymbols()
	for k, v := range rawSymbols {
		s.symbols[k] = &Symbol{v}
	}
}

func GetSymbolService() SymbolService {
	symbolServiceOnce.Do(func() {
		if !symbolServiceInitialized {
			symbolServiceInstance = &symbolService{}
			symbolServiceInstance.init()
			symbolServiceInitialized = true
		}
	})

	return symbolServiceInstance
}
