package config

type Config struct {
	Username          string
	Password          string
	SecuredMetricsAPI bool
	ServerHTTPPort    string

	CloudflareAPIToken   string
	CloudflareZoneID     string
	CloudflareRecordID   string
	CloudflareRecordType string
	CloudflareTTL        int
	CloudflareProxied    bool

	CronIPUpdateEnable               bool
	CronIPUpdateInitialDelay         int // in seconds
	CronIPUpdateInterval             int // in seconds
	CronPublicIpAPI                  string
	CronPublicIpAPIResponseAttribute string
	CronHostname                     string
}
