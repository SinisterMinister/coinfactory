package binance

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

type ServerTime struct {
	Timestamp int `json:"serverTime"`
}

// ExchangeInfo contains the response from the exchangeInfo API call
type ExchangeInfo struct {
	Timezone   string       `json:"timezone"`
	ServerTime int          `json:"serverTime"`
	RateLimits []RateLimit  `json:"rateLimits"`
	Symbols    []SymbolData `json:"symbols"`
}

// RateLimit contains the rate limit settings of the API
type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	Limit         int    `json:"limit"`
}

// SymbolData contains the data for the symbol retrieved with the exchange info
type SymbolData struct {
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

// AllMarketTickersStreamHandler handles incoming data from GetAllMarketTickersStream
type AllMarketTickersStreamHandler interface {
	ReceiveData(payload []SymbolTickerData)
}

type UserDataStreamHandler interface {
	ReceiveData(payload UserDataPayload)
}

type AccountUpdatePayload struct {
	EventTime            int                   `json:"E"`
	MakerCommissionRate  decimal.Decimal       `json:"m"`
	TakerCommisionRate   decimal.Decimal       `json:"t"`
	BuyerCommissionRate  decimal.Decimal       `json:"b"`
	SellerCommissionRate decimal.Decimal       `json:"s"`
	CanTrade             bool                  `json:"T"`
	CanWithdraw          bool                  `json:"W"`
	CanDeposit           bool                  `json:"D"`
	LastUpdate           int                   `json:"u"`
	Balances             []StreamWalletBalance `json:"B"`
}

type OrderUpdatePayload struct {
	EventTime             int             `json:"E"`
	Symbol                string          `json:"s"`
	ClientOrderID         string          `json:"c"`
	Side                  string          `json:"S"`
	OrderType             string          `json:"o"`
	TimeInForce           string          `json:"f"`
	OrderQuantity         decimal.Decimal `json:"q"`
	OrderPrice            decimal.Decimal `json:"p"`
	StopPrice             decimal.Decimal `json:"P"`
	IcebergQuantity       decimal.Decimal `json:"F"`
	OriginalClientOrderID string          `json:"C"`
	CurrentExecutionType  string          `json:"x"`
	CurrentOrderStatus    string          `json:"X"`
	RejectReason          string          `json:"r"`
	OrderID               int             `json:"i"`
	LastExecutedQuantity  decimal.Decimal `json:"l"`
	FilledQuantity        decimal.Decimal `json:"z"`
	LastExecutedPrice     decimal.Decimal `json:"L"`
	CommissionAmmount     decimal.Decimal `json:"n"`
	CommissionAsset       string          `json:"N"`
	TransactionTime       int             `json:"T"`
	TradeID               int             `json:"t"`
	IsWorking             bool            `json:"w"`
	IsMaker               bool            `json:"m"`
}

type UserDataPayload struct {
	AccountUpdatePayload AccountUpdatePayload
	OrderUpdatePayload   OrderUpdatePayload
}

func (udp *UserDataPayload) UnmarshalJSON(b []byte) error {
	var tmp interface{}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	switch t := tmp.(map[string]interface{})["e"]; t {
	case "outboundAccountInfo":
		if err := strictTagsUnmarshal(b, &udp.AccountUpdatePayload); err != nil {
			return err
		}
	case "executionReport":
		if err := strictTagsUnmarshal(b, &udp.OrderUpdatePayload); err != nil {
			return err
		}
	}
	return nil
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

type StreamWalletBalance struct {
	Asset  string          `json:"a"`
	Free   decimal.Decimal `json:"f"`
	Locked decimal.Decimal `json:"l"`
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

type KlineRequest struct {
	Symbol    string `json:"symbol"`
	Interval  string `json:"interval"`
	Limit     int    `json:"limit,omitempty"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
}

type Kline struct {
	OpenTime         time.Time
	CloseTime        time.Time
	OpenPrice        decimal.Decimal
	ClosePrice       decimal.Decimal
	LowPrice         decimal.Decimal
	HighPrice        decimal.Decimal
	BaseVolume       decimal.Decimal
	QuoteVolume      decimal.Decimal
	TradeCount       int
	TakerAssetVolume decimal.Decimal
	TakerQuoteVolume decimal.Decimal
}

func (k *Kline) UnmarshalJSON(b []byte) error {
	var (
		tmp interface{}
		err error
	)
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	openInNano := tmp.([]interface{})[0].(float64) * 1000000
	k.OpenTime = time.Unix(0, int64(openInNano))
	k.OpenPrice, err = decimal.NewFromString(tmp.([]interface{})[1].(string))
	k.HighPrice, err = decimal.NewFromString(tmp.([]interface{})[2].(string))
	k.LowPrice, err = decimal.NewFromString(tmp.([]interface{})[3].(string))
	k.ClosePrice, err = decimal.NewFromString(tmp.([]interface{})[4].(string))
	k.BaseVolume, err = decimal.NewFromString(tmp.([]interface{})[5].(string))
	closeInNano := tmp.([]interface{})[6].(float64) * 1000000
	k.CloseTime = time.Unix(0, int64(closeInNano))
	k.QuoteVolume, err = decimal.NewFromString(tmp.([]interface{})[7].(string))
	k.TradeCount = int(tmp.([]interface{})[8].(float64))
	k.TakerAssetVolume, err = decimal.NewFromString(tmp.([]interface{})[9].(string))
	k.TakerQuoteVolume, err = decimal.NewFromString(tmp.([]interface{})[10].(string))

	return err
}

type ResponseError struct {
	msg string
	res *http.Response
}

func (e ResponseError) Error() string            { return e.msg }
func (e ResponseError) Response() *http.Response { return e.res }
