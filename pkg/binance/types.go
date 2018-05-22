package binance

import (
	"encoding/json"
	"net/http"

	"github.com/shopspring/decimal"
)

type ServerTime struct {
	Timestamp int `json:"serverTime"`
}

// ExchangeInfo contains the response from the exchangeInfo API call
type ExchangeInfo struct {
	Timezone   string      `json:"timezone"`
	ServerTime int         `json:"serverTime"`
	RateLimits []RateLimit `json:"rateLimits"`
	Symbols    []Symbol    `json:"symbols"`
}

// RateLimit contains the rate limit settings of the API
type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	Limit         int    `json:"limit"`
}

// Symbol contains the data for the symbol retrieved with the exchange info
type Symbol struct {
	Symbol             string                 `json:"symbol"`
	Status             string                 `json:"status"`
	BaseAsset          string                 `json:"baseAsset"`
	BaseAssetPrecision int                    `json:"baseAssetPrecision"`
	QuoteAsset         string                 `json:"quoteAsset"`
	QuotePrecision     int                    `json:"quotePrecision"`
	OrderTypes         []string               `json:"orderTypes"`
	IcebergAllowed     bool                   `json:"icebergAllowed"`
	Filters            SymbolFilterCollection `json:"filters"`
}

type SymbolFilterCollection struct {
	Price           PriceFilter
	LotSize         LotSizeFilter
	MinimumNotional MinimumNotionalFilter
}

func (sfc *SymbolFilterCollection) UnmarshalJSON(b []byte) error {
	// need a temporary collection that we can use to determine the filter type
	type tempFilter struct {
		FilterType string `json:"filterType"`
	}

	// Extract into interface slice
	tmp := []interface{}{}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	// Iterate over and exctract the filters
	for _, f := range tmp {
		switch a := f.(map[string]interface{})["filterType"].(string); a {
		case "PRICE_FILTER":
			sfc.Price.MinPrice, _ = decimal.NewFromString(f.(map[string]interface{})["minPrice"].(string))
			sfc.Price.MaxPrice, _ = decimal.NewFromString(f.(map[string]interface{})["maxPrice"].(string))
			sfc.Price.TickSize, _ = decimal.NewFromString(f.(map[string]interface{})["tickSize"].(string))
		case "LOT_SIZE":
			sfc.LotSize.MinQuantity, _ = decimal.NewFromString(f.(map[string]interface{})["minQty"].(string))
			sfc.LotSize.MaxQuantity, _ = decimal.NewFromString(f.(map[string]interface{})["maxQty"].(string))
			sfc.LotSize.StepSize, _ = decimal.NewFromString(f.(map[string]interface{})["stepSize"].(string))
		case "MIN_NOTIONAL":
			sfc.MinimumNotional.MinNotional, _ = decimal.NewFromString(f.(map[string]interface{})["minNotional"].(string))
		}
	}
	return nil
}

type PriceFilter struct {
	MinPrice decimal.Decimal `json:"minPrice"`
	MaxPrice decimal.Decimal `json:"maxPrice"`
	TickSize decimal.Decimal `json:"tickSize"`
}

type LotSizeFilter struct {
	MinQuantity decimal.Decimal `json:"minQty"`
	MaxQuantity decimal.Decimal `json:"maxQty"`
	StepSize    decimal.Decimal `json:"stepSize"`
}

type MinimumNotionalFilter struct {
	MinNotional decimal.Decimal `json:"minNotional"`
}

// SymbolTickerData contains the ticker data from a stream
type SymbolTickerData struct {
	Symbol               string          `json:"s"`
	PriceChange          decimal.Decimal `json:"p"`
	PriceChangePercent   string          `json:"P"`
	WeightedAveragePrice decimal.Decimal `json:"w"`
	PreviousClosePrice   decimal.Decimal `json:"x"`
	CurrentClosePrice    decimal.Decimal `json:"c"`
	CloseTradeQuantity   string          `json:"Q"`
	BidPrice             decimal.Decimal `json:"b"`
	BidQuantity          decimal.Decimal `json:"B"`
	AskPrice             decimal.Decimal `json:"a"`
	AskQuantity          decimal.Decimal `json:"A"`
	OpenPrice            decimal.Decimal `json:"o"`
	HighPrice            decimal.Decimal `json:"h"`
	LowPrice             decimal.Decimal `json:"l"`
	BaseVolume           decimal.Decimal `json:"v"`
	QuoteVolume          decimal.Decimal `json:"q"`
	OpenTime             int             `json:"O"`
	CloseTime            int             `json:"C"`
	FirstTradeID         int             `json:"F"`
	LastTradeID          int             `json:"L"`
	TotalNumberOfTrades  int             `json:"n"`
}

// GetAllMarketTickersStreamHandler handles incoming data from GetAllMarketTickersStream
type GetAllMarketTickersStreamHandler interface {
	ReceiveData(payload []SymbolTickerData)
}

type OrderRequest struct {
	Symbol          string          `json:"symbol"`
	Side            string          `json:"side"`
	Type            string          `json:"type"`
	TimeInForce     string          `json:"timeInForce,omitempt"`
	Quantity        decimal.Decimal `json:"quantity"`
	Price           decimal.Decimal `json:"price,omitempty"`
	ClientOrderID   string          `json:"newClientOrderId,omitempty"`
	StopPrice       decimal.Decimal `json:"stopPrice,omitempty"`
	IcebergQuantity decimal.Decimal `json:"icebergQty,omitempty"`
	ResponseType    string          `json:"newOrderRespType,omitempty"`
	ReceiveWindow   int             `json:"recvWindow,omitempty"`
}

type OrderResponseAckResponse struct {
	Symbol          string `json:"symbol"`
	OrderID         int    `json:"orderId"`
	ClientOrderID   string `json:"clientOrderId"`
	TransactionTime int    `json:"transactTime"`
}

type OrderResponseResultResponse struct {
	OrderResponseAckResponse
	Price            decimal.Decimal `json:"price"`
	OriginalQuantity decimal.Decimal `json:"origQty"`
	ExecutedQuantity decimal.Decimal `json:"executedQty"`
	Status           string          `json:"status"`
	TimeInForce      string          `json:"timeInForce"`
	Type             string          `json:"type"`
	Side             string          `json:"side"`
}

type OrderStatusRequest struct {
	Symbol        string `json:"symbol"`
	OrderID       int    `json:"orderId,omitempty"`
	ClientOrderID string `json:"clientOrderId,omitempty"`
	ReceiveWindow int    `json:"recvWindow,omitempty"`
}

type OrderStatusResponse struct {
	Symbol           string          `json:"symbol"`
	OrderID          int             `json:"orderId"`
	ClientOrderID    string          `json:"clientOrderId"`
	Price            decimal.Decimal `json:"price"`
	OriginalQuantity decimal.Decimal `json:"origQty"`
	ExecutedQuantity decimal.Decimal `json:"executedQty"`
	Status           string          `json:"status"`
	TimeInForce      string          `json:"timeInForce"`
	Type             string          `json:"type"`
	Side             string          `json:"side"`
	StopPrice        string          `json:"stopPrice"`
	IcebergQuantity  decimal.Decimal `json:"icebergQty"`
	Timestamp        int             `json:"TIME"`
	IsWorking        bool            `json:"isWorking"`
}

type OrderCancellationRequest struct {
	Symbol                string `json:"symbol"`
	OrderID               int    `json:"orderId,omitempty"`
	OriginalClientOrderID string `json:"origClientOrderId,omitempty"`
	NewClientOrderID      string `json:"newClientOrderId,omitempty"`
	ReceiveWindow         int    `json:"recvWindow,omitempty"`
}

type OrderCancellationResponse struct {
	Symbol                string `json:"symbol"`
	OrderID               int    `json:"orderId"`
	OriginalClientOrderID string `json:"origClientOrderId"`
	ClientOrderID         string `json:"clientOrderId"`
}

type ResponseError struct {
	msg string
	res *http.Response
}

type UserDataResponse struct {
	MakerCommission  int             `json:"makerCommission"`
	TakerCommission  int             `json:"takerCommission"`
	BuyerCommission  int             `json:"buyerCommission"`
	SellerCommission int             `json:"sellerCommission"`
	CanTrade         bool            `json:"canTrade"`
	CanWithdraw      bool            `json:"canWithdraw"`
	CanDeposit       bool            `json:"canDeposit"`
	UpdateTime       int             `json:"updateTime"`
	Balances         []WalletBalance `json:"balances"`
}

type WalletBalance struct {
	Asset  string          `json:"asset"`
	Free   decimal.Decimal `json:"free"`
	Locked decimal.Decimal `json:"locked"`
}

type TradeRequest struct {
	Symbol        string `json:"symbol"`
	Limit         int    `json:"limit,omitempty"`
	FromTradeID   int    `json:"fromId,omitempty"`
	ReceiveWindow int    `json:"recvWindow,omitempty"`
}

type Trade struct {
	ID              int             `json:"id"`
	OrderID         int             `json:"orderId"`
	Price           decimal.Decimal `json:"price"`
	Quantity        decimal.Decimal `json:"qty"`
	Commission      decimal.Decimal `json:"commission"`
	CommissionAsset string          `json:"commissionAsset"`
	Time            int             `json:"time"`
	IsBuyer         bool            `json:"isBuyer"`
	IsMaker         bool            `json:"isMaker"`
	IsBestMatch     bool            `json:"isBestMatch"`
}

type ListenKeyPayload struct {
	ListenKey string `json:"listenKey"`
}

func (e ResponseError) Error() string            { return e.msg }
func (e ResponseError) Response() *http.Response { return e.res }
