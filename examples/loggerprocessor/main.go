package main

import (
	"os"
	"os/signal"

	"github.com/sinisterminister/coinfactory"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

// LoggerSymbolTickerStreamProcessor implements the SymbolStreamProcessor interface.
type LoggerSymbolTickerStreamProcessor struct{}

// ProcessData takes the data from Binance's 24hr symbol ticker stream and logs the watched symbol's data to the command line.
func (processor *LoggerSymbolTickerStreamProcessor) ProcessData(data binance.SymbolTickerData) {
	log.WithFields(log.Fields{
		"data": data,
	}).Info("Ticker for " + data.Symbol)
}

// processorFactory implements the SymbolStreamProcessorFactory contract which allows Coinfactory to
// create processors for each symbol
func processorFactory(symbol binance.Symbol) coinfactory.SymbolStreamProcessor {
	proc := LoggerSymbolTickerStreamProcessor{}
	return &proc
}

func main() {
	cf := coinfactory.NewCoinFactory(processorFactory)
	cf.Start()

	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:
		cf.Stop()
	}
}
