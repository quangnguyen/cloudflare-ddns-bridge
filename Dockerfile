FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION
ENV VERSION=${VERSION}

RUN go build -o /app/bin/cloudflare-ddns-bridge -ldflags="-X main.version=$VERSION -v -s" ./cmd/main.go

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/bin/cloudflare-ddns-bridge /bin

ENTRYPOINT ["/bin/cloudflare-ddns-bridge"]