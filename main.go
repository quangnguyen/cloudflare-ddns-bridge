package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var version = "dev"

type Config struct {
	Username       string
	Password       string
	ServerHTTPPort string

	CloudflareAPIToken   string
	CloudflareZoneID     string
	CloudflareRecordID   string
	CloudflareRecordType string
	CloudflareProxied    string
}

type DNSRecordUpdate struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied,omitempty"`
}

type CloudflareDNSResponse struct {
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

func loadConfig() (*Config, error) {
	requiredEnv := []string{"USERNAME", "PASSWORD", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ZONE_ID", "CLOUDFLARE_RECORD_ID"}
	config := &Config{
		Username:             os.Getenv("USERNAME"),
		Password:             os.Getenv("PASSWORD"),
		CloudflareAPIToken:   os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareZoneID:     os.Getenv("CLOUDFLARE_ZONE_ID"),
		CloudflareRecordID:   os.Getenv("CLOUDFLARE_RECORD_ID"),
		CloudflareRecordType: "A",    // Default value
		CloudflareProxied:    "true", // Default value
	}

	for _, env := range requiredEnv {
		if value := os.Getenv(env); value == "" {
			return nil, fmt.Errorf("missing required environment variable: %s", env)
		}
	}

	if recordType := os.Getenv("CLOUDFLARE_RECORD_TYPE"); recordType != "" {
		config.CloudflareRecordType = recordType
	}

	if proxied := os.Getenv("CLOUDFLARE_PROXIED"); proxied != "" {
		config.CloudflareProxied = proxied
	}

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	config.ServerHTTPPort = fmt.Sprintf(":%s", port)

	return config, nil
}

func updateCloudflareDNSRecord(cfg *Config, update DNSRecordUpdate) (*CloudflareDNSResponse, error) {
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

	var response CloudflareDNSResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK || !response.Success {
		return nil, fmt.Errorf("cloudflare API error (status %d): %v", resp.StatusCode, response.Errors)
	}

	return &response, nil
}

func authenticate(cfg *Config, next http.HandlerFunc) http.HandlerFunc {
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

func handleDNSUpdate(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hostname := r.URL.Query().Get("hostname")
		ipAddress := r.URL.Query().Get("myip")

		if hostname == "" || ipAddress == "" {
			http.Error(w, "Missing required parameters: hostname and myip", http.StatusBadRequest)
			return
		}

		if net.ParseIP(ipAddress) == nil {
			http.Error(w, "Invalid IP address format", http.StatusBadRequest)
			return
		}

		update := DNSRecordUpdate{
			Type:    cfg.CloudflareRecordType,
			Name:    hostname,
			Content: ipAddress,
			Proxied: cfg.CloudflareProxied == "true",
		}

		response, err := updateCloudflareDNSRecord(cfg, update)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error updating DNS record: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response.Result); err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
		}
	}
}

func main() {
	fmt.Println("Cloudflare DDNS bridge version:", version)

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/nic/update", authenticate(cfg, handleDNSUpdate(cfg)))

	server := &http.Server{
		Addr:         cfg.ServerHTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Printf("Starting cloudflare bridge server on %s", cfg.ServerHTTPPort)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server error: %v", err)
	}
}
