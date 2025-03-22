package cloudflare

import (
	"bytes"
	"cloudflare-dns-bridge/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DNSRecordUpdate struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied,omitempty"`
}

type DNSResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Content string `json:"content"`
		Proxied bool   `json:"proxied"`
		TTL     int    `json:"ttl"`
	} `json:"result"`
}

func UpdateCloudflareDNSRecord(cfg *config.Config, update DNSRecordUpdate) (*DNSResponse, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	requestBody, err := json.Marshal(update)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %w", err)
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", cfg.CloudflareZoneID, cfg.CloudflareRecordID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.CloudflareAPIToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %w", err)
	}
	defer resp.Body.Close()

	var response DNSResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || !response.Success {
		return nil, fmt.Errorf("cloudflare API error (status %d): %v", resp.StatusCode, response.Errors)
	}

	return &response, nil
}
