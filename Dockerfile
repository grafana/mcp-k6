# Build stage
FROM golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

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

# Build the application and k6 from the same upgraded module graph
RUN make build && go build -mod=mod -o k6 go.k6.io/k6/v2

# Final stage
FROM grafana/k6:latest-with-browser@sha256:584354da6a3c1d9ac71c8accc3609443c51d78faf0c55fa49d3a84015e5eaa87

LABEL io.modelcontextprotocol.server.name="io.github.grafana/mcp-k6"

USER root

RUN apk upgrade --no-cache

# Set the working directory (k6 image uses /home/k6)
WORKDIR /home/k6

# Copy the binary from the builder stage (k6 user has UID 12345)
COPY --from=builder --chown=12345:12345 /app/mcp-k6 /home/k6/
COPY --from=builder /app/k6 /usr/bin/k6

# Use the k6 user (already exists in the k6 image)
USER k6

# Run the mcp-k6 application instead of k6
ENTRYPOINT ["/home/k6/mcp-k6"]

# Expose port 8080 for Streamable HTTP transport
EXPOSE 8080 
