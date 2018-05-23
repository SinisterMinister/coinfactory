package coinfactory

import (
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type symbolStreamProcessorWrapper struct {
	processor   SymbolStreamProcessor
	dataChannel chan binance.SymbolTickerData
	quitChannel chan bool
}

func (wrapper *symbolStreamProcessorWrapper) kill() {
	wrapper.quitChannel <- true
}

// symbolTickerStreamHandler
type symbolTickerStreamHandler struct {
	processors       map[string]symbolStreamProcessorWrapper
	processorFactory SymbolStreamProcessorFactory
	mux              *sync.Mutex
	doneChannel      chan bool
}

func (handler *symbolTickerStreamHandler) start() {
	handler.refreshProcessors()

	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info("Config updated. Refreshing processors...")
		handler.refreshProcessors()
	})

	handler.doneChannel = binance.GetAllMarketTickersStream(handler)
}

func (handler *symbolTickerStreamHandler) stop() {
	// Kill the handler
	handler.doneChannel <- true

	for _, p := range handler.processors {
		p.kill()
	}
}

// ReceiveData takes the payload from the stream and forwards it to the correct processor
func (handler *symbolTickerStreamHandler) ReceiveData(payload []binance.SymbolTickerData) {
	for _, sym := range payload {
		handler.mux.Lock()
		if proc, ok := handler.processors[sym.Symbol]; ok {
			proc.dataChannel <- sym
		}
		handler.mux.Unlock()
	}
}

func (hanlder *symbolTickerStreamHandler) refreshProcessors() {
	// Fetch eligible symbols
	symbols := fetchWatchedSymbols()

	// Lock down everything
	hanlder.mux.Lock()
	defer hanlder.mux.Unlock()

	// Remove any uncalled for processors
filterLoop:
	for s, p := range hanlder.processors {
		for _, sym := range symbols {
			if strings.Contains(sym, s) {
				continue filterLoop
			}
		}
		log.Info("Processor for " + s + " is exiting")
		p.kill()
		delete(hanlder.processors, s)
	}

	// Create any missing processors
	for _, s := range symbols {
		if _, ok := hanlder.processors[s]; !ok {
			// Start the processor
			proc := hanlder.processorFactory(binance.GetSymbol(s))

			log.Info("Processor for " + s + " is starting")
			hanlder.processors[s] = newSymbolStreamProcessorWrapper(proc)
		}
	}
}

func newSymbolStreamProcessorWrapper(processor SymbolStreamProcessor) symbolStreamProcessorWrapper {
	dc := make(chan binance.SymbolTickerData)
	qc := make(chan bool)
	wrapper := symbolStreamProcessorWrapper{
		processor:   processor,
		dataChannel: dc,
		quitChannel: qc,
	}

	wrapper.start()

	return wrapper
}

func (wrapper *symbolStreamProcessorWrapper) start() {
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

func newSymbolTickerStreamHandler(factory SymbolStreamProcessorFactory) *symbolTickerStreamHandler {
	handler := symbolTickerStreamHandler{}
	handler.processorFactory = factory
	handler.mux = &sync.Mutex{}
	handler.processors = make(map[string]symbolStreamProcessorWrapper)

	return &handler
}
