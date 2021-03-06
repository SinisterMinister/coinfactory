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
	symbolCache       map[string]SymbolData
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

func setSymbolCache(cache map[string]SymbolData) {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	symbolCache = cache
}

func buildSymbolCache(info ExchangeInfo) map[string]SymbolData {
	symbols := make(map[string]SymbolData)

	for _, s := range info.Symbols {
		symbols[s.Symbol] = s
	}

	return symbols
}

func setServerTimeDelta(info ExchangeInfo) {
	serverTimeDelta = calculateServerTimeDelta(info.ServerTime)
}

func getSymbolsAsStrings() []string {
	symbolStrings := []string{}
	symbols := GetSymbols()

	for s := range symbols {
		symbolStrings = append(symbolStrings, s)
	}

	return symbolStrings
}

func placeOrderGetResult(order OrderRequest) (OrderResponseResultResponse, error) {
	var response OrderResponseResultResponse
	err := placeOrder(order, &response)
	if err != nil {
		return OrderResponseResultResponse{}, err
	}
	return response, nil
}

func placeOrderGetAck(order OrderRequest) (OrderResponseAckResponse, error) {
	var response OrderResponseAckResponse
	// Make sure response type is ack
	order.ResponseType = "ACK"
	err := placeOrder(order, &response)
	if err != nil {
		return OrderResponseAckResponse{}, err
	}
	return response, nil
}
