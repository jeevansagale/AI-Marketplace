# Marketplace API Gateway — build image

FROM golang:1.22-alpine AS build

WORKDIR /src

# Copy module files
COPY shared/catalog/go.mod shared/catalog/
COPY shared/catalog/*.go shared/catalog/

COPY apps/gateway/go.mod apps/gateway/

# Disable the root go.work during the container build.
# The gateway is built as its own Go module.
ENV GOWORK=off

# Download dependencies
RUN cd apps/gateway && go mod download

# Copy gateway source
COPY apps/gateway/ apps/gateway/

# Build
RUN cd apps/gateway && \
    CGO_ENABLED=0 \
    go build -o /out/gateway ./cmd/gateway


# Runtime image
FROM alpine:3.19

RUN adduser -D -H marketplace && \
    apk add --no-cache ca-certificates

USER marketplace

COPY --from=build /out/gateway /usr/local/bin/gateway

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/gateway"]