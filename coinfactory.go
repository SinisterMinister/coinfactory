package coinfactory

import (
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const appName = "coinfactory"

type Coinfactory struct {
	processorFactory SymbolStreamProcessorFactory
	stsHandler       *symbolTickerStreamHandler
}

func (cf *Coinfactory) Start() {
	cf.startSymbolStreamProcessor()
}

func (cf *Coinfactory) GetBalanceManager() BalanceManager {
	return getBalanceManagerInstance()
}

func (cf *Coinfactory) GetOrderManager() OrderManager {
	return getOrderManagerInstance()
}

func (cf *Coinfactory) startSymbolStreamProcessor() {
	handler := cf.stsHandler
	handler.refreshProcessors()

	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info("Config updated. Refreshing processors...")
		handler.refreshProcessors()
	})

	binance.GetAllMarketTickersStream(handler)
}

func NewCoinFactory(ssp SymbolStreamProcessorFactory) Coinfactory {
	getBalanceManagerInstance()
	return Coinfactory{ssp, newSymbolTickerStreamHandler(ssp)}
}

func init() {
	// Tell AWS to use the credentials file for region
	os.Setenv("AWS_SDK_LOAD_CONFIG", "true")

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
