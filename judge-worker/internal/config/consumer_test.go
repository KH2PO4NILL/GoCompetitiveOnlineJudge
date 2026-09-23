package config

import "testing"

func TestConsumerBounds(t *testing.T) {
	valid := ConsumerConf{Enabled: true, Endpoint: "127.0.0.1:18081"}
	valid.defaults()
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ConsumerConf){
		func(config *ConsumerConf) { config.Endpoint = "http://invalid" },
		func(config *ConsumerConf) { config.TimeoutSeconds = 0 },
		func(config *ConsumerConf) { config.VisibilitySeconds = 1 },
		func(config *ConsumerConf) { config.MaxBackoffSeconds = 0 },
		func(config *ConsumerConf) { config.MaxAttempts = 101 },
	} {
		invalid := valid
		mutate(&invalid)
		if invalid.validate() == nil {
			t.Fatal("unbounded consumer accepted")
		}
	}
}
