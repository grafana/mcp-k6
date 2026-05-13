# Build stage
FROM golang:1.26-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder

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
FROM grafana/k6:latest-with-browser@sha256:f07f174bdf5852b3819e55edd0ef0906d3d32d285d4b4750fc763d70a83ee80e

LABEL io.modelcontextprotocol.server.name="io.github.grafana/mcp-k6"

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
