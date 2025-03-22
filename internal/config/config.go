package config

type Config struct {
	Username       string
	Password       string
	ServerHTTPPort string

	CloudflareAPIToken   string
	CloudflareZoneID     string
	CloudflareRecordID   string
	CloudflareRecordType string
	CloudflareTTL        int
	CloudflareProxied    bool
}
