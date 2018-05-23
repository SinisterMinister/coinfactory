package coinfactory

import (
	"github.com/sinisterminister/coinfactory/pkg/binance"
)

// SymbolStreamProcessor is the interface for stream processors
type UserDataStreamProcessor interface {
	ProcessUserData(data binance.UserDataPayload)
}

type UserDataStreamProcessorFactory func() UserDataStreamProcessor
