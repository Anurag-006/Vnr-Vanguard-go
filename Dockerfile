# ==========================================
# STAGE 1: BUILDER
# ==========================================
FROM golang:1.22-alpine AS builder

# Set working directory
WORKDIR /app

# Copy dependency files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary securely
# CGO_ENABLED=0 ensures a static binary that runs anywhere
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vanguard ./cmd/vanguard/main.go

# ==========================================
# STAGE 2: RUNTIME (The tiny production image)
# ==========================================
FROM alpine:latest

# Add ca-certificates so your app can securely make HTTPS requests to the VNR portal and Upstash
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/vanguard .

# CRITICAL: Copy your frontend files! 
COPY --from=builder /app/static ./static

# Expose the default port
EXPOSE 8080

# Command to run the executable
CMD ["./vanguard"]