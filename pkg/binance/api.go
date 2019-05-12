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
	symbolStrings := []string{}
	symbols := GetSymbols()

	for s := range symbols {
		symbolStrings = append(symbolStrings, s)
	}

	return symbolStrings
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
	var response OrderResponseResultResponse
	err := placeOrder(order, &response)
	if err != nil {
		return OrderResponseResultResponse{}, err
	}
	return response, nil
}

func PlaceOrderGetAck(order OrderRequest) (OrderResponseAckResponse, error) {
	var response OrderResponseAckResponse
	err := placeOrder(order, &response)
	if err != nil {
		return OrderResponseAckResponse{}, err
	}
	return response, nil
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

func GetAllMarketTickersStream(handler AllMarketTickersStreamHandler) chan bool {
	return getAllMarketTickersStream(handler)
}

func GetUserDataStream(listenKey ListenKeyPayload, handler UserDataStreamHandler) chan bool {
	return getUserDataStream(listenKey, handler)
}

func GetKlines(request KlineRequest) ([]Kline, error) {
	return getKlines(request)
}

func GetOpenOrders(request OpenOrdersRequest) ([]OrderStatusResponse, error) {
	return getOpenOrders(request)
}
