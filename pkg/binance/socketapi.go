package binance

import (
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func openSocket(path string) *websocket.Conn {
	// TODO: Fix this naive impl with something robust that can handle connection drops
	u := url.URL{Scheme: "wss", Host: viper.GetString("binance.stream_host") + ":" + viper.GetString("binance.stream_port"), Path: path}
	log.Info("Opening socket connection to ", u.String())

	connection, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	log.Info("Connection to ", u.String(), " established!")

	return connection
}

// GetAllMarketTickersStream opens a stream that receives all symbol tickers every second.
func GetAllMarketTickersStream(handler GetAllMarketTickersStreamHandler) {
	// Open the websocket
	conn := openSocket("/ws/!ticker@arr")

	// Close the connection when the function exits
	defer log.Info("Closing socket connection...")
	defer conn.Close()

	// Channel used to exit the handler
	done := make(chan struct{})

	// Intercept the interrupt signal and pass it along
	interrupt = make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Handler closure wrapped in a goroutine
	go func() {
		// Close the channel when the goroutine exits
		defer close(done)

		// Loop forever over the stream
		for {
			// Create a container for the data
			var payload []SymbolTickerData

			// Read the data and handle any errors
			err := conn.ReadJSON(&payload)
			if err != nil {
				log.Error(err)
				return
			}

			// Pass the data to the handler
			handler.ReceiveData(payload)
		}
	}()

	// Codey: Not gonna lie, I don't really understand completely what's going on below. From
	// what I gather, it's just hanging out and gracefully handling any failures or exits.
	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error(err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
