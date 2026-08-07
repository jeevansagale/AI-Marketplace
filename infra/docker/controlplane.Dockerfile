# Marketplace Control Plane — build image
# Requires build context = repository root (includes go.work + shared/).
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work ./
COPY shared/catalog/go.mod shared/catalog/
COPY shared/catalog/*.go shared/catalog/
COPY apps/controlplane/go.mod apps/controlplane/
RUN cd apps/controlplane && go mod download || true
COPY apps/controlplane/ apps/controlplane/
RUN cd apps/controlplane && CGO_ENABLED=0 go build -o /out/controlplane ./cmd/controlplane

# Runtime image
FROM alpine:3.19
RUN adduser -D -H marketplace && apk add --no-cache ca-certificates
USER marketplace
COPY --from=build /out/controlplane /usr/local/bin/controlplane
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/controlplane"]