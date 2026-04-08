package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Local  LocalConfig  `mapstructure:"local"`
	Devices []DeviceConfig `mapstructure:"devices"`
}

type ServerConfig struct {
	URL      string `mapstructure:"url"`
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

type LocalConfig struct {
	Port int `mapstructure:"port"`
}

type DeviceConfig struct {
	NodeID     string         `mapstructure:"node_id"`
	Name       string         `mapstructure:"name"`
	Type       string         `mapstructure:"type"`
	FWVersion  string         `mapstructure:"fw_version"`
	SubDevices []SubDeviceConfig `mapstructure:"sub_devices"`
}

type SubDeviceConfig struct {
	Name    string       `mapstructure:"name"`
	Type    string       `mapstructure:"type"`
	Primary string       `mapstructure:"primary"`
	Params  []ParamConfig `mapstructure:"params"`
}

type ParamConfig struct {
	Name       string      `mapstructure:"name"`
	Type       string      `mapstructure:"type"`
	DataType   string      `mapstructure:"data_type"`
	UIType     string      `mapstructure:"ui_type"`
	Properties []string    `mapstructure:"properties"`
	Bounds     *BoundConfig `mapstructure:"bounds"`
	Default    interface{} `mapstructure:"default"`
}

type BoundConfig struct {
	Min  float64 `mapstructure:"min"`
	Max  float64 `mapstructure:"max"`
	Step float64 `mapstructure:"step"`
}

var AppConfig *Config

func Load(configPath string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath(".")
	}

	viper.SetDefault("local.port", 8090)

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Note: config file not found, using defaults\n")
	}

	AppConfig = &Config{}
	return viper.Unmarshal(AppConfig)
}
