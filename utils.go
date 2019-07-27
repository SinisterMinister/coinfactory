package coinfactory

import (
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/sinisterminister/coinfactory/pkg/binance"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func filterSymbols(symbols []string) []string {
	filter := func(symbol string) bool {
		sym := viper.GetStringSlice("watchedSymbols")
		c := 0
		for _, s := range sym {
			if strings.Contains(symbol, s) {
				c++
			}
		}
		return c >= 2
	}
	filtered := []string{}
	for _, s := range symbols {
		if filter(s) {
			filtered = append(filtered, s)
		}
	}

	return filtered
}

func fetchWatchedSymbols() []string {
	return filterSymbols(binance.GetSymbolsAsStrings())
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
