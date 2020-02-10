package binance

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func openSocket(path string, query map[string]string) *websocket.Conn {
	// TODO: Fix this naive impl with something robust that can handle connection drops
	u := url.URL{Scheme: "wss", Host: viper.GetString("binance.stream_host") + ":" + viper.GetString("binance.stream_port"), Path: path}
	for k, v := range query {
		if u.RawQuery != "" {
			u.RawQuery = u.RawQuery + "&"
		}
		u.RawQuery = u.RawQuery + k + "=" + v
	}
	log.Debug("Opening socket connection to ", u.String())

	connection, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		bodyBytes, err2 := ioutil.ReadAll(resp.Body)
		if err2 != nil {
			log.WithError(err2).Fatal("dial:", err2)
			return nil
		}
		bodyString := string(bodyBytes)
		log.WithError(err).WithField("response", bodyString).Fatal("dial:", err)
	}
	log.Debug("Connection to ", u.String(), " established!")

	return connection
}

func getCombinedTickerStream(stopChan <-chan bool, symbols []string) <-chan []SymbolTickerData {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel for staging data to send
	dataStagingChan := make(chan []SymbolTickerData)
	// Channel used to move data
	dataChan := make(chan []SymbolTickerData)
	// Channel for failure capture
	failChan := make(chan bool)

	// We need to generate the URL based on the requested symbols
	var path []string
	for _, s := range symbols {
		// Add the stream names
		path = append(path, strings.ToLower(s)+"@ticker")
	}

	url := "/stream"
	query := make(map[string]string)

	query["streams"] = strings.Join(path, "/")

	dataHandler := func(done <-chan bool, conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			select {
			case <-done:
				return
			default:
			}

			select {
			case <-done:
				return
			default:
				// Create a container for the data
				var payload CombinedTickerStreamPayload

				// Read the data and handle any errors
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Something bad happened. Time to bail and try again
					log.WithError(err).Error(err)
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
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
				dataStagingChan <- data
			}
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func(doneChan <-chan bool) {
		// Open the websocket
		conn := openSocket(url, query)
		done := make(chan bool)

		// Fire up the data handler
		go dataHandler(done, conn)

		for {
			restartTimer := time.NewTimer(10 * time.Second)
			select {
			case <-doneChan:
				defer conn.Close()
				close(done)

				// Wait for the connection to finish reading
				time.Sleep(2 * time.Second)

				// Close the data handler
				log.Warn("Closing combined ticker stream socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case data := <-dataStagingChan:
				dataChan <- data

			case <-restartTimer.C:
				log.Warn("no combined ticker stream data in 10 seconds. restarting socket")
				failChan <- true
			}
		}
	}

	failHandler := func(doneChan chan bool) {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				close(doneChan)

				doneChan = make(chan bool)

				// Restart the routine
				go socketHandler(doneChan)
			case <-stopChan:
				// Send a done to stop the routine
				close(doneChan)
				return
			}
		}
	}

	// Start up the routine
	go socketHandler(doneChan)
	// Start up the fail handler
	go failHandler(doneChan)

	return dataChan
}

// GetAllMarketTickersStream opens a stream that receives all symbol tickers every second.
func getAllMarketTickersStream(stopChan <-chan bool) <-chan []SymbolTickerData {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel for staging data to send
	dataStagingChan := make(chan []SymbolTickerData)
	// Channel used to move data
	dataChan := make(chan []SymbolTickerData)
	// Channel for failure capture
	failChan := make(chan bool)
	// Socket URL
	url := "/ws/!ticker@arr"

	dataHandler := func(done <-chan bool, conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			select {
			case <-done:
				return
			default:
				// Create a container for the data
				var payload []SymbolTickerData

				// Read the data and handle any errors
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Something bad happened. Time to bail and try again
					log.WithError(err).Error(err)
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
				}

				log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received all market tickers stream data")
				if err := json.Unmarshal(message, &payload); err != nil {
					log.WithError(err).Error("could not understand all market tickers stream data")
					failChan <- true
					return
				}

				// Pass the data to the handler
				dataStagingChan <- payload
			}
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func(doneChan <-chan bool) {
		// Open the websocket
		conn := openSocket(url, nil)
		done := make(chan bool)

		// Fire up the data handler
		go dataHandler(done, conn)

		for {
			restartTimer := time.NewTimer(10 * time.Second)
			select {
			case <-doneChan:
				defer conn.Close()
				close(done)

				// Wait for the connection to finish reading
				time.Sleep(2 * time.Second)

				// Close the data handler
				log.Info("Closing combined ticker stream socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case data := <-dataStagingChan:
				dataChan <- data

			case <-restartTimer.C:
				log.Warn("no combined ticker stream data in 10 seconds. restarting socket")
				failChan <- true
			}
		}
	}

	failHandler := func(doneChan chan bool) {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				close(doneChan)

				doneChan = make(chan bool)

				// Restart the routine
				go socketHandler(doneChan)
			case <-stopChan:
				// Send a done to stop the routine
				close(doneChan)
				return

			case <-interrupt:
				log.Println("interrupt")
				close(doneChan)
				return
			}
		}
	}

	// Start up the routine
	go socketHandler(doneChan)
	// Start up the fail handler
	go failHandler(doneChan)

	return dataChan
}

func getUserDataStream(stopChan <-chan bool) <-chan UserDataPayload {
	// Channel used to exit the handler the first time
	doneChan := make(chan bool)
	// Channel for staging data to send
	dataStagingChan := make(chan UserDataPayload)
	// Channel used to move data
	dataChan := make(chan UserDataPayload)
	// Channel for failure capture
	failChan := make(chan bool)

	dataHandler := func(done <-chan bool, conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			// Bail out on done
			select {
			case <-done:
				return
			default:
			}

			select {
			// Bail out on done
			case <-done:
				return
			default:
				// Create a container for the data
				var payload UserDataPayload

				// Read the data and handle any errors
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Something bad happened. Time to bail and try again
					log.WithError(err).Error("could not receive user data")
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
				}

				log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received user data payload")
				if err := json.Unmarshal(message, &payload); err != nil {
					log.WithError(err).Error("could not parse user data")
					failChan <- true
					return
				}

				// Pass the data to the handler
				dataStagingChan <- payload
			}

		}
	}

	keepAliveHandler := func(done <-chan bool, listenKey ListenKeyPayload) {
		ticker := time.NewTicker(time.Duration(20) * time.Minute)
		for {
			// Bail out on done
			select {
			case <-done:
				return
			default:
			}

			select {
			case <-ticker.C:
				KeepaliveUserDataStream(listenKey)
			case <-done:
				return
			}
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func(doneChan <-chan bool) {
		// Fetch a listen key first
		listenKey, err := CreateUserDataStream()
		if err != nil {
			log.Error(err)
			failChan <- true
			return
		}

		// Open the websocket
		conn := openSocket("/ws/"+listenKey.ListenKey, nil)
		done := make(chan bool)

		// Fire up the data handler
		go dataHandler(done, conn)

		// Start the keepalive handler
		go keepAliveHandler(done, listenKey)

		for {
			restartTimer := time.NewTimer(1 * time.Minute)
			select {
			case <-doneChan:
				defer conn.Close()
				// Close data handler
				close(done)

				// Wait for the connection to finish reading
				time.Sleep(2 * time.Second)

				log.Info("Closing user data socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case payload := <-dataStagingChan:
				select {
				case dataChan <- payload:
				default:
					log.Warn("Skipped blocked user data channel")
				}

			case <-restartTimer.C:
				log.Warn("no user data in 1 minute. restarting socket")
				failChan <- true
			}

		}
	}

	failHandler := func(doneChan chan bool, stopChan <-chan bool) {
		for {
			// Bail out on done
			select {
			case <-doneChan:
				return
			default:
			}

			select {
			case <-failChan:
				// Send a done to stop the routine
				close(doneChan)

				// Channel used to exit the handler
				doneChan = make(chan bool)

				// Restart the routine
				go socketHandler(doneChan)
			case <-stopChan:
				// Send a done to stop the routine
				close(doneChan)
				return
			}
		}
	}

	// Start up the routine
	go socketHandler(doneChan)
	// Start up the fail handler
	go failHandler(doneChan, stopChan)

	return dataChan
}

func getKlineStream(stopChan <-chan bool, ksi KlineSymbolInterval) <-chan KlineStreamPayload {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel for staging data to send
	dataStagingChan := make(chan KlineStreamPayload)
	// Channel used to move data
	dataChan := make(chan KlineStreamPayload)
	// Channel for failure capture
	failChan := make(chan bool)
	// Intercept the interrupt signal and pass it along
	interrupt := make(chan os.Signal, 1)

	// We need to generate the URL based on the requested symbol
	url := "/ws/" + strings.ToLower(ksi.Symbol) + "@kline_" + ksi.Interval

	dataHandler := func(done <-chan bool, conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			select {
			case <-done:
				return
			default:
				// Create a container for the data
				var payload KlineStreamPayload

				// Read the data and handle any errors
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Something bad happened. Time to bail and try again
					log.WithError(err).Error(err)
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
				}

				log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received kline stream data")
				if err := json.Unmarshal(message, &payload); err != nil {
					log.WithError(err).Error("could not receive kline stream data")
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
				}

				// Pass the data to the handler
				dataStagingChan <- payload
			}

		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func(doneChan <-chan bool) {
		// Open the websocket
		conn := openSocket(url, nil)
		done := make(chan bool)

		// Fire up the data handler
		go dataHandler(done, conn)

		for {
			restartTimer := time.NewTimer(30 * time.Second)
			select {
			case d := <-dataStagingChan:
				dataChan <- d

			case <-doneChan:
				defer conn.Close()
				close(done)
				log.Info("Closing kline stream socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case <-restartTimer.C:
				log.Warn("no kline data in 30 seconds. restarting socket")
				failChan <- true
			}
		}
	}

	failHandler := func(doneChan chan bool) {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				close(doneChan)

				// Channel used to exit the handler
				doneChan = make(chan bool)

				// Restart the routine
				go socketHandler(doneChan)
			case <-stopChan:
				// Send a done to stop the routine
				close(doneChan)
				return

			case <-interrupt:
				log.Println("interrupt")
				close(doneChan)
				return
			}
		}
	}

	// Start up the routine
	go socketHandler(doneChan)
	// Start up the fail handler
	go failHandler(doneChan)

	return dataChan
}

func getCombinedKlineStream(stopChan <-chan bool, ksis []KlineSymbolInterval) <-chan []KlineStreamPayload {
	// Channel used to exit the handler
	doneChan := make(chan bool)
	// Channel for staging data to send
	dataStagingChan := make(chan []KlineStreamPayload)
	// Channel used to move data
	dataChan := make(chan []KlineStreamPayload)
	// Channel for failure capture
	failChan := make(chan bool)

	// We need to generate the URL based on the requested symbols
	var path []string
	for _, s := range ksis {
		// Add the stream names
		path = append(path, s.GetStreamName())
	}

	url := "/stream"
	query := make(map[string]string)

	query["streams"] = strings.Join(path, "/")

	dataHandler := func(done <-chan bool, conn *websocket.Conn) {
		// Loop forever over the stream
		for {
			select {
			case <-done:
				return
			default:
			}

			select {
			case <-done:
				return
			default:
				// Create a container for the data
				var payload CombinedKlineStreamPayload

				// Read the data and handle any errors
				_, message, err := conn.ReadMessage()
				if err != nil {
					// Something bad happened. Time to bail and try again
					log.WithError(err).Error(err)
					select {
					case <-done:
						return
					default:
						failChan <- true
						return
					}
				}

				log.WithField("raw paylaod", fmt.Sprintf("%s", message)).Debug("Received combined kline stream data")
				if err := json.Unmarshal(message, &payload); err != nil {
					log.WithError(err).Error("could not receive combined kline stream data")
					failChan <- true
					return
				}

				var data []KlineStreamPayload
				data = append(data, payload.Data)

				// Pass the data to the handler
				dataStagingChan <- data
			}
		}
	}

	// Handler closure wrapped in a goroutine
	socketHandler := func(doneChan <-chan bool) {
		// Open the websocket
		conn := openSocket(url, query)
		done := make(chan bool)

		// Fire up the data handler
		go dataHandler(done, conn)

		for {
			restartTimer := time.NewTimer(10 * time.Second)
			select {
			case <-doneChan:
				defer conn.Close()
				close(done)

				// Wait for the connection to finish reading
				time.Sleep(2 * time.Second)

				// Close the data handler
				log.Warn("Closing combined kline stream socket connection...")
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Error(err)
				}
				return

			case data := <-dataStagingChan:
				dataChan <- data

			case <-restartTimer.C:
				log.Warn("no combined kline stream data in 10 seconds. restarting socket")
				failChan <- true
			}
		}
	}

	failHandler := func(doneChan chan bool) {
		for {
			select {
			case <-failChan:
				// Send a done to stop the routine
				close(doneChan)

				doneChan = make(chan bool)

				// Restart the routine
				go socketHandler(doneChan)
			case <-stopChan:
				// Send a done to stop the routine
				close(doneChan)
				return
			}
		}
	}

	// Start up the routine
	go socketHandler(doneChan)
	// Start up the fail handler
	go failHandler(doneChan)

	return dataChan
}
