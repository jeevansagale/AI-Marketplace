# Marketplace API Gateway — build image
# Requires build context = repository root (includes go.work + shared/).
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work ./
COPY shared/catalog/go.mod shared/catalog/
COPY shared/catalog/*.go shared/catalog/
COPY apps/gateway/go.mod apps/gateway/
RUN cd apps/gateway && go mod download || true
COPY apps/gateway/ apps/gateway/
RUN cd apps/gateway && CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway

# Runtime image
FROM alpine:3.19
RUN adduser -D -H marketplace && apk add --no-cache ca-certificates
USER marketplace
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]