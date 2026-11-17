// Package tclient connects to Temporal the same way for every program in the
// lab.
package tclient

import (
	"log"
	"os"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
	"go.temporal.io/sdk/client"
	sdktally "go.temporal.io/sdk/contrib/tally"
)

// Dial connects to the Temporal frontend named by TEMPORAL_ADDRESS, in the
// namespace named by TEMPORAL_NAMESPACE. When metricsAddr is not empty, the
// SDK's metrics are served on it for Prometheus to scrape.
func Dial(metricsAddr string) (client.Client, error) {
	opts := client.Options{
		HostPort:  Env("TEMPORAL_ADDRESS", "temporal-frontend:7233"),
		Namespace: Env("TEMPORAL_NAMESPACE", "default"),
	}
	if metricsAddr != "" {
		scope, err := newPrometheusScope(metricsAddr)
		if err != nil {
			return nil, err
		}
		opts.MetricsHandler = sdktally.NewMetricsHandler(scope)
	}
	return client.Dial(opts)
}

func newPrometheusScope(addr string) (tally.Scope, error) {
	cfg := prometheus.Configuration{
		ListenAddress: addr,
		TimerType:     "histogram",
	}
	reporter, err := cfg.NewReporter(prometheus.ConfigurationOptions{
		Registry: prom.NewRegistry(),
		OnError: func(err error) {
			log.Println("prometheus reporter error:", err)
		},
	})
	if err != nil {
		return nil, err
	}
	scope, _ := tally.NewRootScope(tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sdktally.PrometheusSanitizeOptions,
	}, time.Second)
	return sdktally.NewPrometheusNamingScope(scope), nil
}

// Env returns the value of an environment variable, or def when it is unset
// or empty.
func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
