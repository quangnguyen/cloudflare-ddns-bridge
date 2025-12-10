# Cloudflare DNS Bridge for FritzBox

A lightweight Go application that acts as a bridge between your FritzBox router and Cloudflare's DNS service, enabling dynamic DNS (DDNS) updates.

## Configuration

### Environment Variables

| Variable Name            | Required | Default | Description                                               |
|--------------------------|----------|---------|-----------------------------------------------------------|
| `CLOUDFLARE_API_TOKEN`   | Yes      | -       | Cloudflare API Token (Zone:DNS:Edit permissions required) |
| `CLOUDFLARE_ZONE_ID`     | Yes      | -       | Cloudflare Zone ID                                        |
| `CLOUDFLARE_RECORD_ID`   | Yes      | -       | DNS Record ID to update                                   |
| `CLOUDFLARE_RECORD_TYPE` | No       | `A`     | DNS Record type (`A` or `AAAA`)                           |
| `CLOUDFLARE_RECORD_TTL`  | No       | `300`   | DNS TTL in seconds (`120-86400`)                          |
| `CLOUDFLARE_PROXIED`     | No       | `false` | Enable Cloudflare proxy (`true`/`false`)                  |
| `USERNAME`               | Yes      | -       | HTTP Basic Auth username                                  |
| `PASSWORD`               | Yes      | -       | HTTP Basic Auth password                                  |
| `SECURED_METRICS_API`    | No       | `true`  | Secure api /metrics with basic auth                       |
| `HTTP_PORT`              | No       | `8080`  | Port to listen on                                         |


## Docker
```shell
docker run -d \
    -p 8080:8080 \
    -e USERNAME=... \
    -e PASSWORD=... \
    -e CLOUDFLARE_API_TOKEN=... \
    -e CLOUDFLARE_ZONE_ID=... \
    -e CLOUDFLARE_RECORD_ID=... \
    lazylab/cloudflare-ddns-bridge:latest
```

## FritzBox Dynamic DNS Configuration

1. Login to your FritzBox admin interface.
2. Navigate to: **Internet > Permit Access > DynDNS**.
3. Configure as follows:
    - **DynDNS Provider:** User-defined
    - **Update-URL:** 
      - Single host: `http://[BRIDGE_IP_OR_HOSTNAME]:[PORT]/nic/update?ip=<ipaddr>&hostname=yourhost.local`
      - Multiple hosts: `http://[BRIDGE_IP_OR_HOSTNAME]:[PORT]/nic/bulk/update?ip=<ipaddr>&hostnames=hostone.local:P,vpn.hostone.local`
    - **Domain name:** Any value (not used by bridge)
    - **Username:** same as `USERNAME` from environment
    - **Password:** same as `PASSWORD` from environment  
