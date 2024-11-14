FROM golang:1.23-alpine

WORKDIR /app

# Install necessary packages
RUN apk add --no-cache \
    tzdata \
    ca-certificates \
    wget

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o backup-service

CMD ["./backup-service"]