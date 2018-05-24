package binance

import (
	"fmt"
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
)

// GetServerTime fetches the API's current time
func getServerTime() (time.Time, error) {
	svrTime := ServerTime{}
	err := unsignedGetJSON(buildURL("/api/v1/time"), &svrTime)
	if err != nil {
		return time.Time{}, err
	}
	// Convert to time.Time and return
	return time.Unix(0, int64(svrTime.Timestamp*1000000)), nil
}

// GetExchangeInfo fetches the exchange info from the API
func getExchangeInfo() (ExchangeInfo, error) {
	info := ExchangeInfo{}
	err := unsignedGetJSON(buildURL("/api/v1/exchangeInfo"), &info)
	if err != nil {
		return ExchangeInfo{}, err
	}

	return info, nil
}

func placeTestOrder(order OrderRequest) error {
	u := buildURL("/api/v3/order/test")
	res, err := signedPost(u, order)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		body, _ := ioutil.ReadAll(res.Body)
		requestBody, _ := ioutil.ReadAll(res.Request.Body)
		log.WithFields(log.Fields{
			"status":      res.Status,
			"statusCode":  res.StatusCode,
			"requestBody": requestBody,
			"body":        fmt.Sprintf("%s", body),
		}).Warn("Error placing test order!")
		return ResponseError{"Error placing test order!", res}
	}

	// No error to return
	return nil
}

func placeOrder(order OrderRequest, response interface{}) error {
	u := buildURL("/api/v3/order")
	res, err := signedPost(u, order)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		body, _ := ioutil.ReadAll(res.Body)
		requestBody, _ := ioutil.ReadAll(res.Request.Body)
		log.WithFields(log.Fields{
			"status":      res.Status,
			"statusCode":  res.StatusCode,
			"requestBody": requestBody,
			"body":        fmt.Sprintf("%s", body),
		}).Warn("Error placing test order!")
		return ResponseError{"Error placing order!", res}
	}

	err = getJSONResponse(res, &response)

	return err
}

func getOrderStatus(order OrderStatusRequest) (OrderStatusResponse, error) {
	var response OrderStatusResponse
	u := buildURL("/api/v3/order")
	// Add payload
	u.RawQuery = structToMap(order).Encode()
	res, err := signedGet(u)
	if err != nil {
		return response, err
	}

	if res.StatusCode >= 400 {
		return response, ResponseError{"Error getting order status!", res}
	}

	// Extract response
	err = getJSONResponse(res, &response)

	return response, err
}

func cancelOrder(order OrderCancellationRequest) (OrderCancellationResponse, error) {
	var response OrderCancellationResponse
	u := buildURL("/api/v3/order")
	u.RawQuery = structToMap(order).Encode()

	res, err := signedDelete(u)
	if err != nil {
		return response, err
	}

	if res.StatusCode >= 400 {
		return response, ResponseError{"Error cancelling order!", res}
	}

	err = getJSONResponse(res, &response)

	return response, err
}

func getTrades(req TradeRequest) ([]Trade, error) {
	var response = []Trade{}
	u := buildURL("/api/v3/myTrades")
	u.RawQuery = structToMap(&req).Encode()
	res, err := signedGet(u)
	if err != nil {
		return response, err
	}

	if res.StatusCode >= 400 {
		return response, ResponseError{"Error getting trades!", res}
	}

	err = getJSONResponse(res, &response)

	return response, err
}

func getUserData() (UserDataResponse, error) {
	var response UserDataResponse
	u := buildURL("/api/v3/account")
	res, err := signedGet(u)
	if err != nil {
		return response, err
	}

	if res.StatusCode >= 400 {
		return response, ResponseError{"Error getting user data!", res}
	}

	err = getJSONResponse(res, &response)

	return response, err
}

func createUserDataStream() (ListenKeyPayload, error) {
	var response ListenKeyPayload
	u := buildURL("/api/v1/userDataStream")
	res, err := unsignedPost(u, nil)
	if err != nil {
		return response, err
	}

	if res.StatusCode >= 400 {
		return response, ResponseError{"Error getting user data!", res}
	}

	err = getJSONResponse(res, &response)

	return response, err
}

func keepaliveUserDataStream(payload ListenKeyPayload) error {
	u := buildURL("/api/v1/userDataStream")
	res, err := unsignedPut(u, nil)

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return ResponseError{"Error getting user data!", res}
	}

	return err
}

func deleteUserDataStream(payload ListenKeyPayload) error {
	u := buildURL("/api/v1/userDataStream")
	res, err := unsignedDelete(u)

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return ResponseError{"Error getting user data!", res}
	}

	return err
}
