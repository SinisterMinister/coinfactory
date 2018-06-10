package coinfactory

import (
	"github.com/sinisterminister/coinfactory/pkg/binance"
)

// SymbolStreamProcessor is the interface for stream processors
type SymbolStreamProcessor interface {
	ProcessData(data binance.SymbolTickerData)
}

type SymbolStreamProcessorFactory func(symbol *Symbol) SymbolStreamProcessor
