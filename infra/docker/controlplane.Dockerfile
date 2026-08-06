# Marketplace Control Plane — build image
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY apps/controlplane/go.mod ./
RUN go mod download || true
COPY apps/controlplane/ ./
RUN CGO_ENABLED=0 go build -o /out/controlplane ./cmd/controlplane

# Runtime image
FROM alpine:3.19
RUN adduser -D -H marketplace && apk add --no-cache ca-certificates
USER marketplace
COPY --from=build /out/controlplane /usr/local/bin/controlplane
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/controlplane"]