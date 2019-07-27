package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/sinisterminister/coinfactory"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Make the kill switch for the various streams
	killSwitch := make(chan bool)

	// Create a list of markets to log
	markets := []string{
		"BTCUSDT",
		"ETHUSDT",
		"XRPUSDT",
	}

	// Create a processor for each ticker and kline stream
	for _, symbol := range markets {
		go tickerProcessor(killSwitch, symbol)

		// Log kline stream
		go klineProcessor(killSwitch, symbol)
	}

	// Let things get warmed up first
	time.Sleep(2 * time.Second)
	coinfactory.Start()

	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:
		// Kill the streams
		close(killSwitch)

		// Shutdown coinfactory
		coinfactory.Shutdown()
	}
}

func tickerProcessor(killSwitch <-chan bool, symbol string) {
	log.Infof("Starting processor for %s market", symbol)
	tickerStream := coinfactory.GetTickerStreamService().GetTickerStream(killSwitch, symbol)

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
}

func klineProcessor(killSwitch <-chan bool, symbol string) {
	log.Infof("Starting processor for %s kline stream", symbol)
	klineStream := coinfactory.GetKlineStreamService().GetKlineStream(killSwitch, symbol, "1m")

	for {
		// Bail on the kill switch
		select {
		case <-killSwitch:
			return
		default:
		}

		// Handle the data
		select {
		case data := <-klineStream:
			log.WithFields(log.Fields{
				"data": data,
			}).Info("Kline for " + data.Symbol)

		// Bail on the kill switch
		case <-killSwitch:
			return
		}
	}
}
