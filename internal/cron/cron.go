package cron

import (
	"cloudflare-dns-bridge/internal/cloudflare"
	"cloudflare-dns-bridge/internal/config"
	"cloudflare-dns-bridge/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	lastDetectedIP string
	ipMutex        sync.RWMutex
)

func StartIPUpdateCron(cfg *config.Config) {
	logger.Logger.Info("Cron - IP update cron job started",
		"initialDelay", cfg.CronIPUpdateInitialDelay,
		"interval", cfg.CronIPUpdateInterval,
		"hostnames", cfg.CronHostnames,
		"ipAPI", cfg.CronPublicIpAPI)

	initialDelay := time.Duration(cfg.CronIPUpdateInitialDelay) * time.Second
	time.AfterFunc(initialDelay, func() {
		updateIPAddress(cfg)
		startRegularUpdates(cfg)
	})
}

func startRegularUpdates(cfg *config.Config) {
	interval := time.Duration(cfg.CronIPUpdateInterval) * time.Second
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			updateIPAddress(cfg)
		}
	}()
}

func updateIPAddress(cfg *config.Config) {
	ipAddress, err := getCurrentIP(cfg)
	if err != nil {
		logger.Logger.Error("Cron - Failed to get current IP", "error", err)
		return
	}

	ipMutex.RLock()
	shouldUpdate := ipAddress != lastDetectedIP
	ipMutex.RUnlock()

	if shouldUpdate {
		for hostname, hostnameConfig := range cfg.CronHostnames {
			update := cloudflare.DNSRecordUpdate{
				Type:    cfg.CloudflareRecordType,
				Name:    hostname,
				Content: ipAddress,
				TTL:     cfg.CloudflareTTL,
				Proxied: hostnameConfig.Proxied,
				ID:      hostnameConfig.RecordID,
			}

			if _, err = cloudflare.UpdateCloudflareDNSRecord(cfg, update); err != nil {
				logger.Logger.Error("Cron - Failed to update Cloudflare DNS", "error", err, "ip", ipAddress)
				return
			}
		}

		ipMutex.Lock()
		lastDetectedIP = ipAddress
		ipMutex.Unlock()
	} else {
		logger.Logger.Info("Cron - IP address is already up to date", "ip", ipAddress)
	}
}

func getCurrentIP(cfg *config.Config) (string, error) {
	resp, err := http.Get(cfg.CronPublicIpAPI)
	if err != nil {
		return "", fmt.Errorf("failed to call IP API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IP API returned non-200 status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	rawResponse := strings.TrimSpace(string(body))

	if cfg.CronPublicIpAPIResponseAttribute == "" {
		if ip := net.ParseIP(rawResponse); ip == nil {
			return "", fmt.Errorf("API returned invalid IP address: %s", rawResponse)
		}
		return rawResponse, nil
	}

	return extractIPFromJSON(body, cfg.CronPublicIpAPIResponseAttribute)
}

func extractIPFromJSON(body []byte, attribute string) (string, error) {
	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return "", fmt.Errorf("failed to parse API response as JSON: %w", err)
	}

	value := getNestedAttribute(jsonResponse, attribute)
	if value == nil {
		return "", fmt.Errorf("attribute '%s' not found in API response", attribute)
	}

	ipStr, ok := value.(string)
	if !ok {
		ipStr = fmt.Sprintf("%v", value)
	}

	ipStr = strings.TrimSpace(ipStr)
	if ip := net.ParseIP(ipStr); ip == nil {
		return "", fmt.Errorf("attribute '%s' does not contain a valid IP address: %s", attribute, ipStr)
	}

	return ipStr, nil
}

// getNestedAttribute supports dot notation for nested JSON attributes
// Example: "data.ip" would look for jsonResponse["data"]["ip"]
func getNestedAttribute(data map[string]interface{}, path string) interface{} {
	keys := strings.Split(path, ".")
	current := interface{}(data)

	for _, key := range keys {
		switch v := current.(type) {
		case map[string]interface{}:
			if next, ok := v[key]; ok {
				current = next
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return current
}
