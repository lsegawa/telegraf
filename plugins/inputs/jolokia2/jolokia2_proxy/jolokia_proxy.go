//go:generate ../../../../tools/readme_config_includer/generator
package jolokia2_proxy

import (
	_ "embed"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs/jolokia2/common"
)

// DO NOT REMOVE THE NEXT TWO LINES! This is required to embed the sampleConfig data.
//
//go:embed sample.conf
var sampleConfig string

type JolokiaProxy struct {
	DefaultFieldPrefix    string `toml:"default_field_prefix"`
	DefaultFieldSeparator string `toml:"default_field_separator"`
	DefaultTagPrefix      string `toml:"default_tag_prefix"`

	URL                   string                     `toml:"url"`
	DefaultTargetPassword string                     `toml:"default_target_password"`
	DefaultTargetUsername string                     `toml:"default_target_username"`
	Targets               []JolokiaProxyTargetConfig `toml:"target"`

	Username        string          `toml:"username"`
	Password        string          `toml:"password"`
	Origin          string          `toml:"origin"`
	ResponseTimeout config.Duration `toml:"response_timeout"`
	tls.ClientConfig

	Metrics  []common.MetricConfig `toml:"metric"`
	client   *common.Client
	gatherer *common.Gatherer
}

type JolokiaProxyTargetConfig struct {
	URL      string `toml:"url"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

func (*JolokiaProxy) SampleConfig() string {
	return sampleConfig
}

func (jp *JolokiaProxy) Gather(acc telegraf.Accumulator) error {
	if jp.gatherer == nil {
		jp.gatherer = common.NewGatherer(jp.createMetrics())
	}

	if jp.client == nil {
		client, err := jp.createClient()

		if err != nil {
			return err
		}

		jp.client = client
	}

	return jp.gatherer.Gather(jp.client, acc)
}

func (jp *JolokiaProxy) createMetrics() []common.Metric {
	var metrics []common.Metric

	for _, metricConfig := range jp.Metrics {
		metrics = append(metrics, common.NewMetric(metricConfig,
			jp.DefaultFieldPrefix, jp.DefaultFieldSeparator, jp.DefaultTagPrefix))
	}

	return metrics
}

func (jp *JolokiaProxy) createClient() (*common.Client, error) {
	proxyConfig := &common.ProxyConfig{
		DefaultTargetUsername: jp.DefaultTargetUsername,
		DefaultTargetPassword: jp.DefaultTargetPassword,
	}

	for _, target := range jp.Targets {
		proxyConfig.Targets = append(proxyConfig.Targets, common.ProxyTargetConfig{
			URL:      target.URL,
			Username: target.Username,
			Password: target.Password,
		})
	}

	return common.NewClient(jp.URL, &common.ClientConfig{
		Username:        jp.Username,
		Password:        jp.Password,
		ResponseTimeout: time.Duration(jp.ResponseTimeout),
		ClientConfig:    jp.ClientConfig,
		ProxyConfig:     proxyConfig,
	})
}
