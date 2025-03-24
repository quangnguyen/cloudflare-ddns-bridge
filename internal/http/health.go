package http

import (
	"cloudflare-dns-bridge/internal/logger"
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

func HealthCheckRequestHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Debug("Healthcheck request",
		"method", r.Method,
		"url", r.URL.String(),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{Status: "OK"})
}
