package coinfactory

import (
	"sync"
	"time"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type userDataStreamProcessorWrapper struct {
	processor   UserDataStreamProcessor
	dataChannel chan binance.UserDataPayload
	quitChannel chan bool
}

func (wrapper *userDataStreamProcessorWrapper) kill() {
	wrapper.quitChannel <- true
}

type userDataStreamHandler struct {
	processors           map[string]userDataStreamProcessorWrapper
	mux                  *sync.Mutex
	streamDoneChannel    chan bool
	keepaliveDoneChannel chan bool
}

var udshOnce = sync.Once{}
var usdhInstance *userDataStreamHandler

func (handler *userDataStreamHandler) start() {
	listenKey, err := binance.CreateUserDataStream()
	if err != nil {
		log.WithError(err).Fatal("Could not create user data stream")
	}
	handler.streamDoneChannel = binance.GetUserDataStream(listenKey, handler)
	handler.keepaliveDoneChannel = make(chan bool)

	go func(done chan bool, listenKey binance.ListenKeyPayload) {
		ticker := time.NewTicker(time.Duration(20) * time.Minute)

		for {
			select {
			case <-ticker.C:
				binance.KeepaliveUserDataStream(listenKey)
			case <-done:
				return
			}
		}
	}(handler.keepaliveDoneChannel, listenKey)
}

func (handler *userDataStreamHandler) stop() {
	// Kill the handler
	handler.keepaliveDoneChannel <- true
	handler.streamDoneChannel <- true

	for _, p := range handler.processors {
		p.kill()
	}
}

// ReceiveData takes the payload from the stream and forwards it to the correct processor
func (handler *userDataStreamHandler) ReceiveData(payload binance.UserDataPayload) {
	handler.mux.Lock()
	defer handler.mux.Unlock()
	for _, proc := range handler.processors {
		proc.dataChannel <- payload
	}
}

func (handler *userDataStreamHandler) registerProcessor(name string, proc UserDataStreamProcessor) {
	if _, ok := handler.processors[name]; !ok {
		wrapper := newUserDataStreamProcessorWrapper(proc)
		handler.processors[name] = wrapper
		return
	}
	log.Fatal("Processer with name '" + name + "' already registered!")
}

func newUserDataStreamProcessorWrapper(processor UserDataStreamProcessor) userDataStreamProcessorWrapper {
	dc := make(chan binance.UserDataPayload)
	qc := make(chan bool)
	wrapper := userDataStreamProcessorWrapper{
		processor:   processor,
		dataChannel: dc,
		quitChannel: qc,
	}

	wrapper.start()
	return wrapper
}

func (wrapper *userDataStreamProcessorWrapper) start() {
	// Launch the goroutine
	go func() {
		for {
			select {
			case data := <-wrapper.dataChannel:
				wrapper.processor.ProcessUserData(data)
			case <-wrapper.quitChannel:
				return
			}
		}
	}()
}

func newUserDataStreamHandler() *userDataStreamHandler {
	handler := userDataStreamHandler{}
	handler.mux = &sync.Mutex{}
	handler.processors = make(map[string]userDataStreamProcessorWrapper)

	return &handler
}

func getUserDataStreamHandlerInstance() *userDataStreamHandler {
	udshOnce.Do(func() {
		usdhInstance = newUserDataStreamHandler()
	})

	return usdhInstance
}
