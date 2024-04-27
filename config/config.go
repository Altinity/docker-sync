package config

import (
	"errors"

	"github.com/spf13/viper"
)

var keys = make(map[string]*Key)

// InitConfig initializes the application's configuration system. It loads
// settings from a specified file, environment variables, or search paths, and
// listens for changes to dynamically update the configuration.
func InitConfig(cfgFile string) error {
	viper.SetEnvPrefix("CHGUARD")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("$HOME/.chguard")
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}
	}

	return nil
}
