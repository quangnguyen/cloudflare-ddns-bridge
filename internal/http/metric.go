package http

import (
	"cloudflare-dns-bridge/internal/config"
	"cloudflare-dns-bridge/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"sync"
)

var (
	totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests received",
		},
		[]string{"path"},
	)
	ipChangeCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ip_change_count",
		Help: "Count of IP changes detected",
	})
	currentIPGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "current_ip",
			Help: "Representing the current IP address",
		},
		[]string{"ip"},
	)

	currentIP string
	mu        sync.Mutex
)

func init() {
	prometheus.MustRegister(totalRequests, ipChangeCount, currentIPGauge)
}

func HealthMetricsRequestHandler(cfg *config.Config) http.HandlerFunc {
	if cfg.SecuredMetricsAPI {
		return authenticate(cfg, handleMetricsRequest)
	}

	return handleMetricsRequest
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Debug("Metrics request",
		"method", r.Method,
		"url", r.URL.String(),
	)

	promhttp.Handler().ServeHTTP(w, r)
}
