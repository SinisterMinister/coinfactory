package coinfactory

import (
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const appName = "coinfactory"

type Coinfactory struct {
	userDataStreamSvc *userDataStreamService
	tickerStreamSvc   *tickerStreamService
}

func (cf *Coinfactory) Start() {
	go cf.userDataStreamSvc.start()
	go cf.tickerStreamSvc.start()
}

func (cf *Coinfactory) Shutdown() {
	cf.userDataStreamSvc.stop()
	cf.tickerStreamSvc.stop()
}

func (cf *Coinfactory) GetBalanceManager() BalanceManager {
	return getBalanceManagerInstance()
}

func (cf *Coinfactory) GetOrderManager() OrderManager {
	return getOrderManagerInstance()
}

func (cf *Coinfactory) GetTickerStreamService() TickerStreamService {
	return getTickerStreamService()
}

func (cf *Coinfactory) GetUserDataStreamService() UserDataStreamService {
	return getUserDataStreamService()
}

func NewCoinFactory() Coinfactory {
	// getBalanceManagerInstance()
	return Coinfactory{getUserDataStreamService(), getTickerStreamService()}
}

func init() {
	// Setup the paths where Viper will search for the config file
	viper.SetConfigName("." + appName)
	viper.AddConfigPath("/etc/" + appName)
	viper.AddConfigPath("$HOME/." + appName)
	viper.AddConfigPath("$HOME")

	// Set the defaults
	setConfigDefaults()

	// Read in the config
	log.Info("Loading configuration file...")
	err := viper.ReadInConfig()

	// If we can't read the config file, we can't do anything. Bail out!
	if err != nil {
		log.Panic("Could not load config file! ", err)
	}
	setLogLevelFromConfig()
	log.Debug("Configuration file values: ", viper.GetViper)

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		setLogLevelFromConfig()
	})
}

func setConfigDefaults() {
	viper.SetDefault("orderUpdateInterval", 15)
}

func setLogLevelFromConfig() {
	switch viper.GetString("logLevel") {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
