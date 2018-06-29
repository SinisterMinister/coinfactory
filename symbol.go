package coinfactory

import (
	"time"

	"github.com/VividCortex/ewma"
	"github.com/sinisterminister/coinfactory/pkg/binance"
)

type Symbol struct {
	binance.SymbolData
	Ticker binance.SymbolTickerData
}

// func (s *Symbol) GetTickerStream() {}

// func (s *Symbol) GetTradeStream() {}

// func (s *Symbol) GetAggregateTradeStream() {}

// func (s *Symbol) GetKLineStream(interval string) {}

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

func (s *Symbol) GetCurrentTrixIndicator(interval string, periods float64) (ma float64, oscillator float64, err error) {
	singleSmoothedValues := []float64{}
	doubleSmoothedValues := []float64{}
	tripleSmoothedValues := []float64{}

	singleSmoothed := ewma.NewMovingAverage(periods)
	doubleSmoothed := ewma.NewMovingAverage(periods)
	tripleSmoothed := ewma.NewMovingAverage(periods)

	// Get the klines to work with
	klines, err := s.GetKLines(interval, time.Time{}, time.Time{}, int(periods*2*3+2))
	if err != nil {
		return ma, oscillator, err
	}

	// Calculate the single smoothed moving average values
	for _, kline := range klines {
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
	return s.Ticker
}
