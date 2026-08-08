# Marketplace Control Plane — build image

FROM golang:1.22-alpine AS build

WORKDIR /src

# Copy module files
COPY shared/catalog/go.mod shared/catalog/
COPY shared/catalog/*.go shared/catalog/

COPY apps/controlplane/go.mod apps/controlplane/

# Do not use the repository-level go.work inside Docker.
ENV GOWORK=off

# Download dependencies
RUN cd apps/controlplane && go mod download

# Copy controlplane source
COPY apps/controlplane/ apps/controlplane/

# Build
RUN cd apps/controlplane && \
    CGO_ENABLED=0 \
    go build -o /out/controlplane ./cmd/controlplane


# Runtime image
FROM alpine:3.19

RUN adduser -D -H marketplace && \
    apk add --no-cache ca-certificates

USER marketplace

COPY --from=build /out/controlplane /usr/local/bin/controlplane

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/controlplane"]