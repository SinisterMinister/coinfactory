package main

import (
	"os"
	"os/signal"

	"github.com/sinisterminister/coinfactory"
	log "github.com/sirupsen/logrus"
)

func main() {
	cf := coinfactory.NewCoinFactory()

	// Make the kill switch for the various streams
	killSwitch := make(chan bool)

	// Create a list of markets to log
	markets := []string{
		"BTCUSDT",
		"ETHUSDT",
		"XRPUSDT",
	}

	// Create a processor for each ticker stream
	for _, symbol := range markets {
		go func(symbol string) {
			log.Infof("Starting processor for %s market", symbol)
			tickerStream := cf.GetTickerStreamService().GetTickerStream(killSwitch, symbol)

			for {
				// Bail on the kill switch
				select {
				case <-killSwitch:
					return
				default:
				}

				// Handle the data
				select {
				case data := <-tickerStream:
					log.WithFields(log.Fields{
						"data": data,
					}).Info("Ticker for " + data.Symbol)

				// Bail on the kill switch
				case <-killSwitch:
					return
				}
			}
		}(symbol)

	}
	cf.Start()

	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:
		// Kill the streams
		close(killSwitch)

		// Shutdown coinfactory
		cf.Shutdown()
	}
}
