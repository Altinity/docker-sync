package telemetry

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/Altinity/docker-sync/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// startPrometheusServer initializes and starts a Prometheus HTTP metrics server.
func startPrometheusServer(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	address := config.TelemetryMetricsPrometheusAddress.String()
	path := config.TelemetryMetricsPrometheusPath.String()

	logger.Info().Str("address", address).Str("path", path).Msg("Starting Prometheus server")

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	server := &http.Server{
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	return server.ListenAndServe()
}
