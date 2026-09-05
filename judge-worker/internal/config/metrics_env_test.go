package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadMetricsEnvironmentOverrides(t *testing.T) {
	for _, test := range []struct {
		name        string
		yamlEnabled bool
		enabled     string
		address     string
		wantEnabled bool
		wantError   string
	}{
		{name: "yaml disabled"},
		{name: "yaml enabled", yamlEnabled: true, wantEnabled: true},
		{name: "enable", enabled: "true", address: "127.0.0.1:19100", wantEnabled: true},
		{name: "disable", yamlEnabled: true, enabled: "false"},
		{name: "invalid boolean", enabled: "private-invalid-value", wantError: "JUDGE_METRICS_ENABLED must be a boolean"},
		{name: "invalid address", enabled: "true", address: "missing-port", wantError: "Metrics.ListenOn"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JUDGE_CONSUMER_ENABLED", "")
			t.Setenv("JUDGE_METRICS_ENABLED", test.enabled)
			t.Setenv("JUDGE_METRICS_LISTEN_ON", test.address)
			settings := validConfig(t)
			settings.Metrics.Enabled = test.yamlEnabled
			content, err := yaml.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(writeConfig(t, string(content)))
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || strings.Contains(err.Error(), "private-invalid-value") {
					t.Fatalf("unexpected validation error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantAddress := DefaultMetricsListenOn
			if test.address != "" {
				wantAddress = test.address
			}
			if loaded.Metrics.Enabled != test.wantEnabled || loaded.Metrics.ListenOn != wantAddress {
				t.Fatalf("unexpected metrics configuration: %+v", loaded.Metrics)
			}
		})
	}
}
