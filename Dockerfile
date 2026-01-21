# Build stage
FROM golang:1.23-alpine AS builder

# Install git for fetching dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Run stage
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Copy migrations directory if needed by your app at runtime
COPY --from=builder /app/migrations ./migrations

# Expose port
EXPOSE 8080

# Command to run
CMD ["./main"]
