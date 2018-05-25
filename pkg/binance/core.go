package binance

import (
	"os"
	"os/signal"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	exchangeInfoCache ExchangeInfo
	symbolCache       map[string]Symbol
	serverTimeDelta   time.Duration
	cacheMux          *sync.Mutex
	interrupt         chan os.Signal
)

func init() {
	// Setup the defaults
	viper.SetDefault("binance.stream_host", "stream.binance.com")
	viper.SetDefault("binance.stream_port", "9443")
	viper.SetDefault("binance.rest_url", "https://api.binance.com")
	cacheMux = &sync.Mutex{}

	startExchangeInfoFetcher()
}

func startExchangeInfoFetcher() {
	// Get first call and bail on failure
	info, err := getExchangeInfo()
	if err != nil {
		log.Fatal("Could not fetch exchange info! ", err)
	}
	refreshFromInfo(info)
	go func() {
		// Start a ticker for polling
		ticker := time.NewTicker(time.Minute)

		// Intercept the interrupt signal and pass it along
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		for {
			select {
			case <-ticker.C:
				info, err := getExchangeInfo()
				if err != nil {
					log.Error("Could not fetch exchange info! ", err)
				}

				refreshFromInfo(info)
			case <-interrupt:
				ticker.Stop()
				return
			}
		}
	}()
}

func refreshFromInfo(info ExchangeInfo) {
	setExchangeInfoCache(info)
	setSymbolCache(buildSymbolCache(info))
	setServerTimeDelta(info)
}

func setExchangeInfoCache(info ExchangeInfo) {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	exchangeInfoCache = info
}

func setSymbolCache(cache map[string]Symbol) {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	symbolCache = cache
}

func buildSymbolCache(info ExchangeInfo) map[string]Symbol {
	symbols := make(map[string]Symbol)

	for _, s := range info.Symbols {
		symbols[s.Symbol] = s
	}

	return symbols
}

func setServerTimeDelta(info ExchangeInfo) {
	serverTimeDelta = calculateServerTimeDelta(info.ServerTime)
}
