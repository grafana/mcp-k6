# Build stage

FROM golang:1.26.2-alpine@sha256:f85330846cde1e57ca9ec309382da3b8e6ae3ab943d2739500e08c86393a21b1 AS builder


RUN apk update && apk upgrade --no-cache
# Install build dependencies
RUN apk add --no-cache make bash git

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN make build

# Final stage
FROM grafana/k6:1.7.1-with-browser@sha256:be705b9a01364034907f4dbf8a6d6b8f0cd959051f08b1ce3383744abe7d1a00
LABEL io.modelcontextprotocol.server.name="io.github.grafana/mcp-k6"

# Mitigate CVE-2026-31789 (OpenSSL) by updating to patched Alpine packages.
USER root
#RUN apk update && apk upgrade --no-cache libcrypto3 libssl3
RUN apk update && apk upgrade --no-cache

# Set the working directory (k6 image uses /home/k6)
WORKDIR /home/k6

# Copy the binary from the builder stage (k6 user has UID 12345)
COPY --from=builder --chown=12345:12345 /app/mcp-k6 /home/k6/

# Use the k6 user (already exists in the k6 image)
USER k6

# Run the mcp-k6 application instead of k6
ENTRYPOINT ["/home/k6/mcp-k6"]

# Expose port 8080 for Streamable HTTP transport
EXPOSE 8080 
