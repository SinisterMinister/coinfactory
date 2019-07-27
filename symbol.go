package coinfactory

import (
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
)

type Symbol struct {
	binance.SymbolData
	ticker                 binance.SymbolTickerData
	trixKlineCache         []binance.Kline
	trixKlineCacheInterval string
	klineStreamStopChannel chan bool
	tickerStreamStopChan   <-chan bool
	tickerMutex            *sync.RWMutex
}

func newSymbol(data binance.SymbolData, stopChan <-chan bool) *Symbol {
	symbol := &Symbol{
		SymbolData:           data,
		tickerMutex:          &sync.RWMutex{},
		tickerStreamStopChan: stopChan,
	}

	go symbol.tickerUpdater()
	return symbol
}

func (s *Symbol) GetTickerStream(stopChan <-chan bool) <-chan binance.SymbolTickerData {
	return getTickerStreamService().GetTickerStream(stopChan, s.Symbol)
}

func (s *Symbol) GetKLineStream(stopChan <-chan bool, interval string) <-chan binance.KlineStreamData {
	return getKlineStreamService().GetKlineStream(stopChan, s.Symbol, interval)
}

func (s *Symbol) GetKLines(interval string, start time.Time, end time.Time, limit int) ([]binance.Kline, error) {
	req := binance.KlineRequest{
		Symbol:   s.Symbol,
		Interval: interval,
	}
	if !start.IsZero() {
		req.StartTime = start.UnixNano() / int64(time.Millisecond)
	}

	if !end.IsZero() {
		req.EndTime = end.UnixNano() / int64(time.Millisecond)
	}

	if limit != 0 {
		req.Limit = limit
	}

	return binance.GetKlines(req)
}

func (s *Symbol) updateTrixKlineCache(interval string, periods float64) (err error) {
	if s.trixKlineCache == nil || s.trixKlineCacheInterval != interval {
		s.trixKlineCache = make([]binance.Kline, 0)
		s.trixKlineCacheInterval = interval
		if s.klineStreamStopChannel != nil {
			close(s.klineStreamStopChannel)
		}
		s.klineStreamStopChannel = make(chan bool)

		// Get the klines to work with
		s.trixKlineCache, err = s.GetKLines(interval, time.Time{}, time.Time{}, int(periods*2*3+2))
		if err != nil {
			return err
		}

		// Start a stream to get updates
		go func() {
			stream := s.GetKLineStream(s.klineStreamStopChannel, interval)
			for {
				select {
				case data := <-stream:
					if data.Closed {
						// Build a Kline
						kline := binance.Kline{
							OpenTime:         time.Unix(0, data.OpenTime*1000000),
							CloseTime:        time.Unix(0, data.CloseTime*1000000),
							OpenPrice:        data.OpenPrice,
							ClosePrice:       data.ClosePrice,
							LowPrice:         data.LowPrice,
							HighPrice:        data.HighPrice,
							BaseVolume:       data.BaseVolume,
							QuoteVolume:      data.QuoteVolume,
							TradeCount:       data.TradeCount,
							TakerAssetVolume: data.TakerAssetVolume,
							TakerQuoteVolume: data.TakerQuoteVolume,
						}
						s.trixKlineCache = append(s.trixKlineCache[1:], kline)
						log.WithField("kline", kline).Debug("Updated Kline cache")
					}
				case <-s.klineStreamStopChannel:
					return
				}
			}
		}()
	}

	return err
}

func (s *Symbol) GetCurrentTrixIndicator(interval string, periods float64) (ma float64, oscillator float64, err error) {
	singleSmoothedValues := []float64{}
	doubleSmoothedValues := []float64{}
	tripleSmoothedValues := []float64{}

	singleSmoothed := ewma.NewMovingAverage(periods)
	doubleSmoothed := ewma.NewMovingAverage(periods)
	tripleSmoothed := ewma.NewMovingAverage(periods)

	// Update the kline cache
	s.updateTrixKlineCache(interval, periods)

	// Calculate the single smoothed moving average values
	for _, kline := range s.trixKlineCache {
		price, _ := kline.ClosePrice.Float64()
		singleSmoothed.Add(price)
		if singleSmoothed.Value() != 0.0 {
			singleSmoothedValues = append(singleSmoothedValues, singleSmoothed.Value())
		}
	}

	// Calculate the double smoothed moving average values
	for _, s := range singleSmoothedValues {
		doubleSmoothed.Add(s)
		if doubleSmoothed.Value() != 0.0 {
			doubleSmoothedValues = append(doubleSmoothedValues, doubleSmoothed.Value())
		}
	}

	// Calculate the triple smoothed moving average values
	for _, s := range doubleSmoothedValues {
		tripleSmoothed.Add(s)
		if tripleSmoothed.Value() != 0.0 {
			tripleSmoothedValues = append(tripleSmoothedValues, tripleSmoothed.Value())
		}
	}

	ma = tripleSmoothed.Value()
	originalValue := tripleSmoothedValues[len(tripleSmoothedValues)-2]
	oscillator = (ma - originalValue) / originalValue

	return ma, oscillator, err
}

func (s *Symbol) GetTicker() binance.SymbolTickerData {
	s.tickerMutex.RLock()
	defer s.tickerMutex.RUnlock()
	return s.ticker
}

func (s *Symbol) tickerUpdater() {
	tickerStream := getTickerStreamService().GetTickerStream(s.tickerStreamStopChan, s.Symbol)
	for {
		select {
		case <-s.tickerStreamStopChan:
			return
		default:
		}

		select {
		case <-s.tickerStreamStopChan:
			return
		case data := <-tickerStream:
			s.tickerMutex.Lock()
			s.ticker = data
			s.tickerMutex.Unlock()

		}
	}
}
