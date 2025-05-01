package http

import (
	"cloudflare-dns-bridge/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests received",
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(totalRequests)
}

func HealthMetricsRequestHandler(cfg *config.Config) http.HandlerFunc {
	if cfg.SecuredMetricsAPI {
		return authenticate(cfg, handleMetricsRequest)
	}

	return handleMetricsRequest
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}
