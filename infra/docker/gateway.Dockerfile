# Marketplace API Gateway — build image
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY apps/gateway/go.mod ./
RUN go mod download || true
COPY apps/gateway/ ./
RUN CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway

# Runtime image
FROM alpine:3.19
RUN adduser -D -H marketplace && apk add --no-cache ca-certificates
USER marketplace
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]