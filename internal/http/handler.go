package http

import (
	"cloudflare-dns-bridge/internal/cloudflare"
	"cloudflare-dns-bridge/internal/config"
	"cloudflare-dns-bridge/internal/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
)

func DdnsRequestHandlerFunc(cfg *config.Config) http.HandlerFunc {
	return authenticate(cfg, handleDNSUpdate(cfg))
}

func authenticate(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != cfg.Username || pass != cfg.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func handleDNSUpdate(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Logger.Debug("Received request",
			"method", r.Method,
			"url", r.URL.String(),
		)

		for key, values := range r.Header {
			logger.Logger.Debug("Header", "key", key, "values", values)
		}

		username, _, ok := r.BasicAuth()
		if ok {
			logger.Logger.Debug("Basic Auth", "username", username)
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			logger.Logger.Warn("Method not allowed", "method", r.Method)
			return
		}

		// Log query parameters
		queryParams := r.URL.Query()
		for key, values := range queryParams {
			logger.Logger.Debug("Query Parameter", "key", key, "values", values)
		}

		hostname := queryParams.Get("hostname")
		ipAddress := queryParams.Get("ip")

		if hostname == "" || ipAddress == "" {
			http.Error(w, "Missing required parameters: hostname and ip", http.StatusBadRequest)
			logger.Logger.Warn("Missing required parameters", "hostname", hostname, "ipAddress", ipAddress)
			return
		}

		if net.ParseIP(ipAddress) == nil {
			http.Error(w, "Invalid IP address format", http.StatusBadRequest)
			logger.Logger.Warn("Invalid IP address format", "ipAddress", ipAddress)
			return
		}

		proxied := cfg.CloudflareProxied
		proxiedParam := queryParams.Get("proxied")
		if proxiedParam != "" {
			parsed, err := strconv.ParseBool(proxiedParam)
			if err != nil {
				http.Error(w, "Invalid proxied parameter, must be 'true' or 'false'", http.StatusBadRequest)
				logger.Logger.Warn("Invalid proxied parameter", "proxied", proxiedParam)
				return
			}
			proxied = parsed
		}

		ttl := cfg.CloudflareTTL
		ttlParam := queryParams.Get("ttl")
		if ttlParam != "" {
			parsedTTL, err := strconv.Atoi(ttlParam)
			if err != nil || parsedTTL <= 0 {
				http.Error(w, "Invalid ttl parameter, must be a positive integer", http.StatusBadRequest)
				logger.Logger.Warn("Invalid TTL parameter", "ttl", ttlParam)
				return
			}
			ttl = parsedTTL
		}

		update := cloudflare.DNSRecordUpdate{
			Type:    cfg.CloudflareRecordType,
			Name:    hostname,
			Content: ipAddress,
			TTL:     ttl,
			Proxied: proxied,
		}

		logger.Logger.Debug("Prepared DNS update", "update", update)

		response, err := cloudflare.UpdateCloudflareDNSRecord(cfg, update)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error updating DNS record: %v", err), http.StatusInternalServerError)
			logger.Logger.Error("Error updating DNS record", "error", err)
			return
		}

		logger.Logger.Info("Cloudflare response", "response", response.Result)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response.Result); err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			logger.Logger.Error("Error encoding response", "error", err)
		}
	}
}
