package coinfactory

import (
	"strings"
	"sync"
	"time"

	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/shopspring/decimal"
)

type BalanceManager interface {
	// GetBalance returns the account
	GetAvailableBalance(asset string) decimal.Decimal
	GetFrozenBalance(asset string) decimal.Decimal
	GetUsableBalance(asset string) decimal.Decimal
	GetReservedBalance(asset string) decimal.Decimal
	AddReservedBalance(asset string, amount decimal.Decimal)
	SubReservedBalance(asset string, amount decimal.Decimal)
}

type balanceManager struct {
	wallets        map[string]*walletWrapper
	loggerStopChan chan bool
	updateStopChan chan bool
	updateMutex    *sync.Mutex
}

type walletWrapper struct {
	binance.WalletBalance
	mux      *sync.Mutex
	Reserved decimal.Decimal
}

type InsufficientFundsError struct {
	msg string
}

func (e InsufficientFundsError) Error() string { return e.msg }

var balanceManagerInstance *balanceManager
var balanceManagerOnce sync.Once
var walletMux = &sync.Mutex{}

func getBalanceManager() *balanceManager {
	balanceManagerOnce.Do(func() {
		i := &balanceManager{map[string]*walletWrapper{}, nil, nil, &sync.Mutex{}}
		balanceManagerInstance = i
		i.init()
	})
	return balanceManagerInstance
}

func (bm *balanceManager) init() {
	err := bm.refreshWallets()
	if err != nil {
		res, ok := err.(binance.ResponseError)
		if !ok {
			log.WithError(err).Fatal("Could not update wallet balances")
		} else {
			body, ok := res.ResponseBodyString()
			if !ok {
				log.WithError(err).Fatal("Could not update wallet balances")
			}
			log.WithError(err).WithField("response", body).Fatal("Could not update wallet balances")
		}

	}

	bm.loggerStopChan = make(chan bool)
	go bm.logBalances(bm.loggerStopChan)
	// Setup user data stream processor to handle balance managing
	go bm.handleUserDataStream(bm.updateStopChan)
}

func (bm *balanceManager) start() {
	// NOOP
}

func (bm *balanceManager) stop() {
	bm.updateMutex.Lock()
	defer bm.updateMutex.Unlock()

	close(bm.updateStopChan)
	bm.updateStopChan = make(chan bool)
}

func (bm *balanceManager) logBalances(done chan bool) {
	log.Info("starting wallet logger")
	t := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-done:
			return
		case <-t.C:
			log.Info("logging wallet balances")
			for _, s := range viper.GetStringSlice("watchedSymbols") {
				w := bm.getWallet(s)
				w.mux.Lock()

				log.WithFields(log.Fields{
					"total":    w.Free.Add(w.Locked),
					"free":     w.Free,
					"locked":   w.Locked,
					"reserved": w.Reserved,
				}).Info(w.Asset)

				w.mux.Unlock()
			}
		}
	}
}

func (bm *balanceManager) GetAvailableBalance(asset string) decimal.Decimal {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	return wallet.Free
}

func (bm *balanceManager) GetUsableBalance(asset string) decimal.Decimal {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	return wallet.Free.Sub(wallet.Reserved)
}

func (bm *balanceManager) GetFrozenBalance(asset string) decimal.Decimal {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	return wallet.Locked
}

func (bm *balanceManager) GetReservedBalance(asset string) decimal.Decimal {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	return wallet.Reserved
}

func (bm *balanceManager) AddReservedBalance(asset string, amount decimal.Decimal) {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	wallet.Reserved = wallet.Reserved.Add(amount)
}

func (bm *balanceManager) SubReservedBalance(asset string, amount decimal.Decimal) {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	wallet.Reserved = wallet.Reserved.Sub(amount)
}

func (bm *balanceManager) addFreeAmount(asset string, amount decimal.Decimal) {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	wallet.Free = bm.wallets[asset].Free.Add(amount)
}

func (bm *balanceManager) subtractFreeAmount(asset string, amount decimal.Decimal) error {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	if wallet.Free.LessThan(amount) {
		return InsufficientFundsError{"Not enough funds to remove!"}
	}
	wallet.Free = wallet.Free.Sub(amount)
	return nil
}

func (bm *balanceManager) addLockedAmount(asset string, amount decimal.Decimal) {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	wallet.Free = bm.wallets[asset].Locked.Add(amount)
}

func (bm *balanceManager) subtractLockedAmount(asset string, amount decimal.Decimal) error {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()
	if wallet.Locked.LessThan(amount) {
		return InsufficientFundsError{"Not enough funds to remove!"}
	}
	wallet.Free = wallet.Locked.Sub(amount)
	return nil
}

func (bm *balanceManager) freezeAmount(asset string, amount decimal.Decimal) error {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()

	if wallet.Free.LessThan(amount) {
		return InsufficientFundsError{"Not enough funds to freeze!"}
	}
	wallet.Free = wallet.Free.Sub(amount)
	wallet.Locked = wallet.Locked.Add(amount)
	return nil
}

func (bm *balanceManager) unfreezeAmount(asset string, amount decimal.Decimal) error {
	wallet := bm.getWallet(asset)
	wallet.mux.Lock()
	defer wallet.mux.Unlock()

	if wallet.Locked.LessThan(amount) {
		return InsufficientFundsError{"Not enough funds to unfreeze!"}
	}
	wallet.Locked = wallet.Free.Sub(amount)
	wallet.Free = wallet.Locked.Add(amount)
	return nil
}

func (bm *balanceManager) refreshWallets() error {
	log.Debug("Refreshing wallets")
	userData, err := binance.GetUserData()
	if err != nil {
		return err
	}

	for _, b := range userData.Balances {
		w := bm.getWallet(b.Asset)
		w.Asset = b.Asset
		w.Free = b.Free
		w.Locked = b.Locked
	}

	return nil
}

func (bm *balanceManager) getWallet(asset string) *walletWrapper {
	walletMux.Lock()
	defer walletMux.Unlock()

	w, ok := bm.wallets[strings.ToUpper(asset)]
	if !ok {
		w = &walletWrapper{binance.WalletBalance{}, &sync.Mutex{}, decimal.Decimal{}}
		bm.wallets[strings.ToUpper(asset)] = w
	}

	return w
}

func (bm *balanceManager) handleUserDataStream(stopChan <-chan bool) {
	userDataStream := getUserDataStreamService().GetAccountUpdateStream(stopChan)
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		select {
		case <-stopChan:
			return
		case payload := <-userDataStream:
			// Update the wallet balances
			for _, sw := range payload.Balances {
				w := bm.getWallet(sw.Asset)
				w.mux.Lock()
				w.Free = sw.Free
				w.Locked = sw.Locked
				w.mux.Unlock()
			}
		}
	}
}
