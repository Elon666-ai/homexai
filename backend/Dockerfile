# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache tzdata git

# Copy source code
COPY . .
COPY go.mod go.sum ./
RUN go mod download

# Build the application
RUN go build -ldflags="-s -w -X 'main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')'" -o bin/$(BINARY_NAME) $(MAIN_PATH)

# Runtime stage
FROM alpine:latest

WORKDIR /app
USER root
RUN apk --no-cache add ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

ENV TZ=Asia/Shanghai

# Copy binary from builder
COPY --from=builder /app/bin/homexai-server.exe .

# Create uploads directory
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["./homexai-server.exe"]
