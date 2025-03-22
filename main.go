package main

import (
	"cloudflare-dns-bridge/internal/config"
	http2 "cloudflare-dns-bridge/internal/http"
	"cloudflare-dns-bridge/internal/logger"
	"cloudflare-dns-bridge/internal/util"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

var version = "dev"

func main() {
	fmt.Println("Cloudflare ddns bridge version:", version)

	cfg, err := loadConfig()
	if err != nil {
		logger.Logger.Error("Configuration error: %v", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/nic/update", http2.DdnsRequestHandlerFunc(cfg))

	server := &http.Server{
		Addr:         cfg.ServerHTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	logger.Logger.Info("Starting cloudflare bridge server", "port", cfg.ServerHTTPPort)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Logger.Error("Server error: %v", err)
	}
}

func loadConfig() (*config.Config, error) {
	requiredEnv := []string{"USERNAME", "PASSWORD", "CLOUDFLARE_API_TOKEN", "CLOUDFLARE_ZONE_ID", "CLOUDFLARE_RECORD_ID"}
	appConfig := &config.Config{
		Username:             os.Getenv("USERNAME"),
		Password:             os.Getenv("PASSWORD"),
		CloudflareAPIToken:   os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareZoneID:     os.Getenv("CLOUDFLARE_ZONE_ID"),
		CloudflareRecordID:   os.Getenv("CLOUDFLARE_RECORD_ID"),
		CloudflareRecordType: util.GetEnvOrDefault("CLOUDFLARE_RECORD_TYPE", "A"),
		CloudflareTTL:        util.GetEnvAsIntOrDefault("CLOUDFLARE_RECORD_TTL", 300),
		CloudflareProxied:    util.GetEnvAsBoolOrDefault("CLOUDFLARE_PROXIED", true),
		ServerHTTPPort:       fmt.Sprintf(":%s", util.GetEnvOrDefault("HTTP_PORT", "8080")),
	}

	var missingEnv []string
	for _, env := range requiredEnv {
		if os.Getenv(env) == "" {
			missingEnv = append(missingEnv, env)
		}
	}

	if len(missingEnv) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missingEnv)
	}

	return appConfig, nil
}
