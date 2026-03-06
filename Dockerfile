# Build stage
FROM golang:1.26-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder

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
FROM grafana/k6:latest-with-browser@sha256:f44e435069ccea164edb86ff36429ed8cc1489cd57797e9ff52480625fca8cf5

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
