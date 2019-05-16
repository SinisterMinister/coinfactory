package coinfactory

import (
	"sync"

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
	processors        map[string]userDataStreamProcessorWrapper
	mux               *sync.Mutex
	streamDoneChannel chan bool
}

var udshOnce = sync.Once{}
var usdhInstance *userDataStreamHandler

func (handler *userDataStreamHandler) start() {
	log.Info("Starting user data stream handler")

	// Make done channel
	handler.streamDoneChannel = make(chan bool)
	go handler.processDataStream(handler.streamDoneChannel, binance.GetUserDataStream(handler.streamDoneChannel))
}

func (handler *userDataStreamHandler) processDataStream(doneChan <-chan bool, dataChan <-chan binance.UserDataPayload) {
	for {
		select {
		case data := <-dataChan:
			handler.mux.Lock()
			for _, proc := range handler.processors {
				proc.dataChannel <- data
			}
			handler.mux.Unlock()
		case <-doneChan:
			return
		}
	}
}

func (handler *userDataStreamHandler) stop() {
	log.Warn("Stopping user data stream handler")
	defer log.Warn("User data stream handler stopped")
	// Kill the handler
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

// SymbolStreamProcessor is the interface for stream processors
type UserDataStreamProcessor interface {
	ProcessUserData(data binance.UserDataPayload)
}
