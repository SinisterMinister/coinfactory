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

// symbolTickerStreamHandler
type userDataStreamHandler struct {
	processors  map[string]userDataStreamProcessorWrapper
	mux         *sync.Mutex
	doneChannel chan bool
}

var udshOnce = sync.Once{}
var usdhInstance *userDataStreamHandler

func (handler *userDataStreamHandler) start() {
	listenKey, err := binance.CreateUserDataStream()
	if err != nil {
		log.WithError(err).Fatal("Could not create user data stream")
	}
	handler.doneChannel = binance.GetUserDataStream(listenKey, handler)
}

func (handler *userDataStreamHandler) stop() {
	// Kill the handler
	handler.doneChannel <- true

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

func (handler *userDataStreamHandler) registerProcessor(name string, factory UserDataStreamProcessorFactory) {
	if _, ok := handler.processors[name]; !ok {
		proc := factory()
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
				wrapper.processor.ProcessData(data)
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
