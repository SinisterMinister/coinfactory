package coinfactory

import (
	"strings"
	"sync"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type symbolStreamProcessorWrapper struct {
	processor   SymbolStreamProcessor
	dataChannel chan binance.SymbolTickerData
	quitChannel chan bool
}

func (pw *symbolStreamProcessorWrapper) kill() {
	pw.quitChannel <- true
}

// symbolTickerStreamHandler
type symbolTickerStreamHandler struct {
	processors       map[string]symbolStreamProcessorWrapper
	processorFactory SymbolStreamProcessorFactory
	procInit         sync.Once
	mux              *sync.Mutex
}

// ReceiveData takes the payload from the stream and forwards it to the correct processor
func (sts *symbolTickerStreamHandler) ReceiveData(payload []binance.SymbolTickerData) {
	for _, sym := range payload {
		sts.mux.Lock()
		if proc, ok := sts.processors[sym.Symbol]; ok {
			proc.dataChannel <- sym
		}
		sts.mux.Unlock()
	}
}

func (sts *symbolTickerStreamHandler) refreshProcessors() {
	// Initialize the processor map
	sts.procInit.Do(func() {
		sts.processors = make(map[string]symbolStreamProcessorWrapper)
	})

	// Fetch eligible symbols
	symbols := fetchWatchedSymbols()

	// Lock down everything
	sts.mux.Lock()
	defer sts.mux.Unlock()

	// Remove any uncalled for processors
filterLoop:
	for s, p := range sts.processors {
		for _, sym := range symbols {
			if strings.Contains(sym, s) {
				continue filterLoop
			}
		}
		log.Info("Processor for " + s + " is exiting")
		p.kill()
		delete(sts.processors, s)
	}

	// Create any missing processors
	for _, s := range symbols {
		if _, ok := sts.processors[s]; !ok {
			// Start the processor
			proc := sts.processorFactory(binance.GetSymbol(s))

			sts.processors[s] = newSymbolStreamProcessorWrapper(proc)
		}
	}
}

func fetchWatchedSymbols() []string {
	return filterSymbols(binance.GetSymbolsAsStrings())
}

func newSymbolStreamProcessorWrapper(processor SymbolStreamProcessor) symbolStreamProcessorWrapper {
	dc := make(chan binance.SymbolTickerData)
	qc := make(chan bool)
	wrapper := symbolStreamProcessorWrapper{
		processor:   processor,
		dataChannel: dc,
		quitChannel: qc,
	}

	// Launch the goroutine
	go func() {
		for {
			select {
			case data := <-dc:
				processor.ProcessData(data)
			case <-qc:
				return
			}
		}
	}()

	return wrapper
}

func newSymbolTickerStreamHandler(factory SymbolStreamProcessorFactory) *symbolTickerStreamHandler {
	handler := symbolTickerStreamHandler{}
	handler.processorFactory = factory
	handler.mux = &sync.Mutex{}

	return &handler
}
