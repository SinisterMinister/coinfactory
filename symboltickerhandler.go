package coinfactory

import (
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type symbolStreamProcessorWrapper struct {
	symbol      string
	processor   SymbolStreamProcessor
	quitChannel chan bool
}

func (wrapper *symbolStreamProcessorWrapper) kill() {
	close(wrapper.quitChannel)
}

// symbolTickerStreamHandler
type symbolTickerStreamHandler struct {
	processors       map[string]symbolStreamProcessorWrapper
	processorFactory SymbolStreamProcessorFactory
	mux              *sync.Mutex
	doneChannel      chan bool
}

func (handler *symbolTickerStreamHandler) start() {
	log.Info("Starting symbol ticker stream handler")
	defer log.Info("Symbol ticker stream handler started successfully")
	handler.refreshProcessors()

	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info("Config updated. Refreshing processors")
		handler.refreshProcessors()
	})
}

func (handler *symbolTickerStreamHandler) stop() {
	log.Warn("Stopping user data stream handler")
	defer log.Warn("User data stream handler stopped")
	// Kill the handler
	handler.doneChannel <- true

	for _, p := range handler.processors {
		p.kill()
	}
}

func (handler *symbolTickerStreamHandler) refreshProcessors() {
	// Fetch eligible symbols
	symbols := fetchWatchedSymbols()

	// Lock down everything
	handler.mux.Lock()
	defer handler.mux.Unlock()

	// Remove any uncalled for processors
filterLoop:
	for s, p := range handler.processors {
		for _, sym := range symbols {
			if strings.Contains(sym, s) {
				continue filterLoop
			}
		}
		log.Info("Processor for " + s + " is exiting")
		p.kill()
		delete(handler.processors, s)
	}

	// Create any missing processors
	for _, s := range symbols {
		if _, ok := handler.processors[s]; !ok {
			// Start the processor
			symbol, err := GetSymbolService().GetSymbol(s)
			if err != nil {
				continue
			}
			proc := handler.processorFactory(symbol)

			log.Info("Processor for " + s + " is starting")
			handler.processors[s] = newSymbolStreamProcessorWrapper(proc, s)
		}
	}
}

func newSymbolStreamProcessorWrapper(processor SymbolStreamProcessor, symbol string) symbolStreamProcessorWrapper {
	qc := make(chan bool)
	wrapper := symbolStreamProcessorWrapper{
		symbol:      symbol,
		processor:   processor,
		quitChannel: qc,
	}

	wrapper.start()

	return wrapper
}

func (wrapper *symbolStreamProcessorWrapper) start() {
	// Launch the goroutine
	go func() {
		dataChannel := getTickerStreamService().GetTickerStream(wrapper.quitChannel, wrapper.symbol)
		for {
			select {
			case data := <-dataChannel:
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
