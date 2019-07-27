package binance

import "time"

func GetServerTime() (time.Time, error) {
	return getServerTime()
}

func GetExchangeInfo() ExchangeInfo {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	return exchangeInfoCache
}

func GetSymbols() map[string]SymbolData {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	return symbolCache
}

func GetSymbolsAsStrings() []string {
	return getSymbolsAsStrings()
}

func GetSymbol(symbol string) SymbolData {
	cacheMux.Lock()
	defer cacheMux.Unlock()
	return symbolCache[symbol]
}

func PlaceTestOrder(order OrderRequest) error {
	return placeTestOrder(order)
}

func PlaceOrderGetResult(order OrderRequest) (OrderResponseResultResponse, error) {
	return placeOrderGetResult(order)
}

func PlaceOrderGetAck(order OrderRequest) (OrderResponseAckResponse, error) {
	return placeOrderGetAck(order)
}

func GetOrderStatus(order OrderStatusRequest) (OrderStatusResponse, error) {
	return getOrderStatus(order)
}

func CancelOrder(order OrderCancellationRequest) (OrderCancellationResponse, error) {
	return cancelOrder(order)
}

func GetUserData() (UserDataResponse, error) {
	return getUserData()
}

func GetTrades(req TradeRequest) ([]Trade, error) {
	return getTrades(req)
}

func CreateUserDataStream() (ListenKeyPayload, error) {
	return createUserDataStream()
}

func KeepaliveUserDataStream(payload ListenKeyPayload) error {
	return keepaliveUserDataStream(payload)
}

func DeleteUserDataStream(payload ListenKeyPayload) error {
	return deleteUserDataStream(payload)
}

func GetAllMarketTickersStream(stopChan <-chan bool) <-chan []SymbolTickerData {
	return getAllMarketTickersStream(stopChan)
}

func GetCombinedTickerStream(stopChan <-chan bool, symbols []string) <-chan []SymbolTickerData {
	return getCombinedTickerStream(stopChan, symbols)
}

func GetUserDataStream(stopChan <-chan bool) <-chan UserDataPayload {
	return getUserDataStream(stopChan)
}

func GetKlines(request KlineRequest) ([]Kline, error) {
	return getKlines(request)
}

func GetOpenOrders(request OpenOrdersRequest) ([]OrderStatusResponse, error) {
	return getOpenOrders(request)
}

func GetKlineStream(stopChan <-chan bool, ksi KlineSymbolInterval) <-chan KlineStreamPayload {
	return getKlineStream(stopChan, ksi)
}

func GetCombinedKlineStream(stopChan <-chan bool, ksis []KlineSymbolInterval) <-chan []KlineStreamPayload {
	return getCombinedKlineStream(stopChan, ksis)
}
