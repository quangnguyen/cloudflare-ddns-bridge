package http

import (
	"github.com/prometheus/client_golang/prometheus"
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
