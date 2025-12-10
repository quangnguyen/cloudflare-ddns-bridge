package cron

import (
	"cloudflare-dns-bridge/internal/cloudflare"
	"cloudflare-dns-bridge/internal/config"
	"cloudflare-dns-bridge/internal/logger"
	"encoding/json"
	"fmt"
	"io"
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
	logger.Logger.Info("IP update cron job started",
		"initialDelay", cfg.CronIPUpdateInitialDelay,
		"interval", cfg.CronIPUpdateInterval,
		"hostname", cfg.CronHostname,
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
		logger.Logger.Error("Failed to get current IP", "error", err)
		return
	}

	ipMutex.RLock()
	shouldUpdate := ipAddress != lastDetectedIP
	ipMutex.RUnlock()

	if shouldUpdate {
		update := cloudflare.DNSRecordUpdate{
			Type:    cfg.CloudflareRecordType,
			Name:    cfg.CronHostname,
			Content: ipAddress,
			TTL:     cfg.CloudflareTTL,
			Proxied: cfg.CloudflareProxied,
		}

		if _, err = cloudflare.UpdateCloudflareDNSRecord(cfg, update); err != nil {
			logger.Logger.Error("Failed to update Cloudflare DNS", "error", err, "ip", ipAddress)
			return
		}

		ipMutex.Lock()
		lastDetectedIP = ipAddress
		ipMutex.Unlock()
	}
}

func getCurrentIP(cfg *config.Config) (string, error) {
	resp, err := http.Get(cfg.CronPublicIpAPI)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if cfg.CronPublicIpAPIResponseAttribute == "" {
		return strings.TrimSpace(string(body)), nil
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return "", fmt.Errorf("failed to parse API response as JSON: %w", err)
	}

	value, ok := jsonResponse[cfg.CronPublicIpAPIResponseAttribute]
	if !ok {
		return "", fmt.Errorf("attribute '%s' not found in API response", cfg.CronPublicIpAPIResponseAttribute)
	}

	ipValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("attribute '%s' is not a string value", cfg.CronPublicIpAPIResponseAttribute)
	}

	return ipValue, nil
}
