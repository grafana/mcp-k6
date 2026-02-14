# Build stage
FROM golang:1.26-alpine@sha256:d4c4845f5d60c6a974c6000ce58ae079328d03ab7f721a0734277e69905473e5 AS builder

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
FROM grafana/k6:latest-with-browser@sha256:b433632f55ef79968c05eedd3b65afa42eebbbdc3be52c3cac36cecd33ec625b

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
