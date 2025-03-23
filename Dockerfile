FROM golang:1.24-alpine AS builder

WORKDIR /go/src/github.com/quangnguyen/cloudflare-ddns-bridge/

COPY . .

ARG VERSION
ENV VERSION=${VERSION}

RUN go get -v -t .
RUN go build -o bin/app -ldflags="-X main.version=$VERSION -v -s" .

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /go/src/github.com/quangnguyen/cloudflare-ddns-bridge/bin/app /bin

ENTRYPOINT ["/bin/app"]