package http

import (
	"cloudflare-dns-bridge/internal/cloudflare"
	"cloudflare-dns-bridge/internal/config"
	"cloudflare-dns-bridge/internal/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func DdnsRequestBulkHandlerFunc(cfg *config.Config) http.HandlerFunc {
	totalRequests.WithLabelValues("/nic/bulk/update").Inc()
	return authenticate(cfg, handleDNSUpdate(cfg, true))
}

func DdnsRequestHandlerFunc(cfg *config.Config) http.HandlerFunc {
	totalRequests.WithLabelValues("/nic/update").Inc()
	return authenticate(cfg, handleDNSUpdate(cfg, false))
}

func handleDNSUpdate(cfg *config.Config, bulk bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Logger.Debug("Received request",
			"method", r.Method,
			"url", r.URL.String(),
		)

		if err := validateRequestMethod(r); err != nil {
			http.Error(w, err.Error(), http.StatusMethodNotAllowed)
			return
		}

		logRequestDetails(r)

		query := r.URL.Query()
		logQueryParameters(query)

		newIP := query.Get("ip")
		if err := validateIP(newIP); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ttl, err := parseTTL(query.Get("ttl"), cfg.CloudflareTTL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		updates, err := prepareDNSUpdates(cfg, bulk, query, newIP, ttl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		logger.Logger.Debug("Prepared DNS updates",
			"count", len(updates),
			"updates", updates,
		)

		results := processDNSUpdates(cfg, updates)
		updateCurrentIP(newIP)

		w.Header().Set("Content-Type", "application/json")
		sendResponse(w, bulk, updates, results)
	}
}

func validateRequestMethod(r *http.Request) error {
	if r.Method != http.MethodGet {
		logger.Logger.Warn("Method not allowed", "method", r.Method)
		return fmt.Errorf("method not allowed")
	}
	return nil
}

func logRequestDetails(r *http.Request) {
	for key, values := range r.Header {
		logger.Logger.Debug("Header", "key", key, "values", values)
	}

	username, _, ok := r.BasicAuth()
	if ok {
		logger.Logger.Debug("Basic Auth", "username", username)
	}
}

func logQueryParameters(query map[string][]string) {
	for key, values := range query {
		logger.Logger.Debug("Query Parameter", "key", key, "values", values)
	}
}

func validateIP(ip string) error {
	if net.ParseIP(ip) == nil {
		logger.Logger.Warn("Invalid IP address format", "ip", ip)
		return fmt.Errorf("invalid IP address format")
	}
	return nil
}

func parseTTL(ttlParam string, defaultTTL int) (int, error) {
	if ttlParam == "" {
		return defaultTTL, nil
	}

	ttl, err := strconv.Atoi(ttlParam)
	if err != nil || ttl <= 0 {
		logger.Logger.Warn("Invalid TTL parameter", "ttl", ttlParam)
		return 0, fmt.Errorf("invalid ttl parameter, must be a positive integer")
	}

	return ttl, nil
}

func prepareDNSUpdates(cfg *config.Config, bulk bool, query url.Values, newIP string, ttl int) ([]cloudflare.DNSRecordUpdate, error) {
	if !bulk {
		return prepareSingleUpdate(cfg, query, newIP, ttl)
	}
	return prepareBulkUpdates(cfg, query, newIP, ttl)
}

func prepareSingleUpdate(cfg *config.Config, query url.Values, newIP string, ttl int) ([]cloudflare.DNSRecordUpdate, error) {
	hostname := query.Get("hostname")
	if hostname == "" || newIP == "" {
		logger.Logger.Warn("Missing required parameters",
			"hostname", hostname,
			"ip", newIP,
		)
		return nil, fmt.Errorf("missing required parameters: hostname and ip")
	}

	proxied, err := parseProxiedParam(query.Get("proxied"), cfg.CloudflareProxied)
	if err != nil {
		return nil, err
	}

	return []cloudflare.DNSRecordUpdate{
		{
			Type:    cfg.CloudflareRecordType,
			Name:    hostname,
			Content: newIP,
			TTL:     ttl,
			Proxied: proxied,
		},
	}, nil
}

func prepareBulkUpdates(cfg *config.Config, query url.Values, newIP string, ttl int) ([]cloudflare.DNSRecordUpdate, error) {
	hostsParam := query.Get("hosts")
	if hostsParam == "" {
		logger.Logger.Warn("Missing required parameter", "hosts", hostsParam)
		return nil, fmt.Errorf("missing required parameter: hosts")
	}

	return parseBulkHosts(hostsParam, newIP, ttl, cfg)
}

func parseProxiedParam(proxiedParam string, defaultProxied bool) (bool, error) {
	if proxiedParam == "" {
		return defaultProxied, nil
	}

	proxied, err := strconv.ParseBool(proxiedParam)
	if err != nil {
		logger.Logger.Warn("Invalid proxied parameter", "proxied", proxiedParam)
		return false, fmt.Errorf("invalid proxied parameter, must be 'true' or 'false'")
	}

	return proxied, nil
}

func processDNSUpdates(cfg *config.Config, updates []cloudflare.DNSRecordUpdate) []interface{} {
	var results []interface{}

	for _, update := range updates {
		result := processSingleUpdate(cfg, update)
		results = append(results, result)
	}

	return results
}

func processSingleUpdate(cfg *config.Config, update cloudflare.DNSRecordUpdate) interface{} {
	response, err := cloudflare.UpdateCloudflareDNSRecord(cfg, update)
	if err != nil {
		logger.Logger.Error("Error updating DNS record",
			"error", err,
			"hostname", update.Name,
		)
		return map[string]interface{}{
			"hostname": update.Name,
			"success":  false,
			"error":    err.Error(),
		}
	}

	logger.Logger.Info("Cloudflare response",
		"hostname", update.Name,
		"response", response.Result,
	)

	return map[string]interface{}{
		"hostname": update.Name,
		"success":  true,
		"proxied":  update.Proxied,
		"result":   response.Result,
	}
}

func updateCurrentIP(newIP string) {
	if newIP == "" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if newIP != currentIP {
		currentIP = newIP
		ipChangeCount.Inc()
		currentIPGauge.Reset()
		currentIPGauge.WithLabelValues(newIP).Set(1)
	}
}

func sendResponse(w http.ResponseWriter, bulk bool, updates []cloudflare.DNSRecordUpdate, results []interface{}) {
	var responseData interface{}

	switch {
	case bulk || len(updates) > 1:
		responseData = map[string]interface{}{
			"success": true,
			"results": results,
			"count":   len(results),
		}
	case len(results) > 0:
		responseData = results[0]
	default:
		responseData = map[string]interface{}{
			"success": false,
			"error":   "no updates processed",
		}
	}

	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		logger.Logger.Error("Error encoding response", "error", err)
	}
}

// parseBulkHosts parses the hosts parameter in format: "host1:P,host2,host3:P"
func parseBulkHosts(hostsParam, ip string, ttl int, cfg *config.Config) ([]cloudflare.DNSRecordUpdate, error) {
	var updates []cloudflare.DNSRecordUpdate

	hosts := strings.Split(hostsParam, ",")
	for _, hostEntry := range hosts {
		hostEntry = strings.TrimSpace(hostEntry)
		if hostEntry == "" {
			continue
		}

		hostname, proxied := parseHostEntry(hostEntry, cfg.CloudflareProxied)
		if hostname == "" {
			return nil, fmt.Errorf("empty hostname in entry: %s", hostEntry)
		}

		updates = append(updates, cloudflare.DNSRecordUpdate{
			Type:    cfg.CloudflareRecordType,
			Name:    hostname,
			Content: ip,
			TTL:     ttl,
			Proxied: proxied,
		})
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no valid hosts provided")
	}

	return updates, nil
}

func parseHostEntry(hostEntry string, defaultProxied bool) (string, bool) {
	if strings.HasSuffix(hostEntry, ":P") {
		return hostEntry[:len(hostEntry)-2], true
	}
	return hostEntry, defaultProxied
}
