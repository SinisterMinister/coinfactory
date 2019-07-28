package coinfactory

import "time"

const appName = "coinfactory"

func Start() {
	go getUserDataStreamService().start()
	go getTickerStreamService().start()
	go getKlineStreamService().start()
	go getSymbolService().start()
	go getBalanceManager().start()
}

func Shutdown() {
	getUserDataStreamService().stop()
	getTickerStreamService().stop()
	getKlineStreamService().stop()
	getSymbolService().stop()
	getBalanceManager().stop()

	// Let things get warmed up first
	time.Sleep(5 * time.Second)
}

func GetBalanceManager() BalanceManager {
	return getBalanceManager()
}

func GetOrderService() OrderService {
	return getOrderService()
}

func GetTickerStreamService() TickerStreamService {
	return getTickerStreamService()
}

func GetUserDataStreamService() UserDataStreamService {
	return getUserDataStreamService()
}

func GetKlineStreamService() KlineStreamService {
	return getKlineStreamService()
}

func GetSymbolService() SymbolService {
	return getSymbolService()
}
