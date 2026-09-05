package config

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

type ConsumerConf struct {
	Enabled           bool   `yaml:"Enabled"`
	Endpoint          string `yaml:"Endpoint"`
	TLS               bool   `yaml:"TLS"`
	AccessKey         string `yaml:"AccessKey"`
	SecretKey         string `yaml:"SecretKey"`
	VisibilitySeconds int64  `yaml:"VisibilitySeconds"`
	TimeoutSeconds    int64  `yaml:"TimeoutSeconds"`
	BackoffSeconds    int64  `yaml:"BackoffSeconds"`
	MaxBackoffSeconds int64  `yaml:"MaxBackoffSeconds"`
	MaxAttempts       int32  `yaml:"MaxAttempts"`
}

func (config *ConsumerConf) defaults() {
	if config.VisibilitySeconds == 0 {
		config.VisibilitySeconds = 60
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 5
	}
	if config.BackoffSeconds == 0 {
		config.BackoffSeconds = 5
	}
	if config.MaxBackoffSeconds == 0 {
		config.MaxBackoffSeconds = 300
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 16
	}
}
func (config ConsumerConf) validate() error {
	if !config.Enabled {
		return nil
	}
	host, port, err := net.SplitHostPort(config.Endpoint)
	portNumber, portErr := strconv.Atoi(port)
	if portErr != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("invalid Consumer endpoint port")
	}
	if err != nil || strings.TrimSpace(host) == "" || port == "" || config.TimeoutSeconds < 1 || config.TimeoutSeconds > 60 || config.VisibilitySeconds < 15 || config.VisibilitySeconds < 3*config.TimeoutSeconds || config.VisibilitySeconds > 43200 || config.BackoffSeconds < 1 || config.MaxBackoffSeconds < config.BackoffSeconds || config.MaxBackoffSeconds > 43200 || config.MaxAttempts < 1 || config.MaxAttempts > 100 {
		return errors.New("invalid Consumer endpoint, timeout, visibility or retry bounds")
	}
	return nil
}
