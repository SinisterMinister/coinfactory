package binance

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func openSocket(path string, query map[string]string) *websocket.Conn {
	// TODO: Fix this naive impl with something robust that can handle connection drops
	u := url.URL{Scheme: "wss", Host: viper.GetString("binance.stream_host") + ":" + viper.GetString("binance.stream_port"), Path: path}
	q := u.Query()
	for k, v := range query {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	log.Debug("Opening socket connection to ", u.String())

	connection, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.WithError(err).Fatal("dial:", err)
			return nil
		}
		bodyString := string(bodyBytes)
		log.WithError(err).WithField("response", bodyString).Fatal("dial:", err)
	}
	log.Debug("Connection to ", u.String(), " established!")

	return connection
}

func getCombinedTickerStream(symbols []string, handler TickersStreamHandler) chan<- bool {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel used to move data
	dataChan := make(chan []SymbolTickerData)
	// Channel for failure capture
	failChan := make(chan bool)
	// Channel for stopping stream processing
	stopChan := make(chan bool)
	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)

	// We need to generate the URL based on the requested symbols
	var path []string
	for _, s := range symbols {
		// Add the stream names
		path = append(path, strings.ToLower(s)+"@ticker")
	}

	url := "/stream"
	query := make(map[string]string)

	query["streams"] = strings.Join(path, "/")

	dataHandler := func(conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			// Create a container for the data
			var payload CombinedTickerStreamPayload

			// Read the data and handle any errors
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Something bad happened. Time to bail and try again
				log.WithError(err).Error(err)
				failChan <- true
				return
			}

			log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received combined ticker stream data")
			if err := json.Unmarshal(message, &payload); err != nil {
				log.WithError(err).Error("could not receive combined ticker stream data")
				failChan <- true
				return
			}

			var data []SymbolTickerData
			data = append(data, payload.Data)

			// Pass the data to the handler
			dataChan <- data
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func() {
		// Open the websocket
		conn := openSocket(url, query)

		// Fire up the data handler
		go dataHandler(conn)

		for {
			select {
			case <-doneChan:
				defer conn.Close()
				log.Info("Closing combined ticker stream socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case data := <-dataChan:
				handler.ReceiveData(data)
			}

		}
	}

	failHandler := func() {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				doneChan <- true

				// Restart the routine
				go socketHandler()
			case <-stopChan:
				// Send a done to stop the routine
				doneChan <- true
				return

			case <-interrupt:
				log.Println("interrupt")
				doneChan <- true
				return
			}
		}
	}

	// Start up the routine
	go socketHandler()
	// Start up the fail handler
	go failHandler()

	return stopChan
}

// GetAllMarketTickersStream opens a stream that receives all symbol tickers every second.
func getAllMarketTickersStream(handler TickersStreamHandler) chan bool {
	// Open the websocket
	conn := openSocket("/ws/!ticker@arr", nil)

	// Channel used to exit the handler
	done := make(chan bool)

	// Handler closure wrapped in a goroutine
	go func() {
		// Close the connection when the function exits
		defer log.Debug("Closing all market tickers socket connection...")
		defer conn.Close()

		// Close the channel when the goroutine exits
		defer close(done)

		// Loop forever over the stream
		for {
			// Create a container for the data
			var payload []SymbolTickerData

			// Read the data and handle any errors
			err := conn.ReadJSON(&payload)
			if err != nil {
				log.WithError(err).Error("could not read from symbol ticker socket")
				return
			}

			// Pass the data to the handler
			handler.ReceiveData(payload)
		}
	}()

	go func() {
		// Intercept the interrupt signal and pass it along
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)

		for {
			select {
			case <-interrupt:
				log.Println("interrupt")
			case <-done:
			}
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.WithError(err).Error(err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}()

	return done
}

func getUserDataStream(listenKey ListenKeyPayload, handler UserDataStreamHandler) chan<- bool {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel used to move data
	dataChan := make(chan UserDataPayload)
	// Channel for failure capture
	failChan := make(chan bool)
	// Channel for stopping stream processing
	stopChan := make(chan bool)
	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)

	dataHandler := func(conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			// Create a container for the data
			var payload UserDataPayload

			// Read the data and handle any errors
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Something bad happened. Time to bail and try again
				log.WithError(err).Error(err)
				failChan <- true
				return
			}

			log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received user data payload")
			if err := json.Unmarshal(message, &payload); err != nil {
				log.WithError(err).Error("could not receive user data")
				failChan <- true
				return
			}

			// Pass the data to the handler
			dataChan <- payload
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func() {
		// Open the websocket
		conn := openSocket("/ws/"+listenKey.ListenKey, nil)

		// Fire up the data handler
		go dataHandler(conn)

		for {
			select {
			case <-doneChan:
				defer conn.Close()
				log.Info("Closing user data socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case payload := <-dataChan:
				handler.ReceiveData(payload)
			}

		}
	}

	failHandler := func() {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				doneChan <- true

				// Restart the routine
				go socketHandler()
			case <-stopChan:
				// Send a done to stop the routine
				doneChan <- true
				return

			case <-interrupt:
				log.Println("interrupt")
				doneChan <- true
				return
			}
		}
	}

	// Start up the routine
	go socketHandler()
	// Start up the fail handler
	go failHandler()

	return stopChan
}
