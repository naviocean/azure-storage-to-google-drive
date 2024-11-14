FROM golang:1.20-alpine

WORKDIR /app

RUN apk add --no-cache \
    tzdata \
    ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o restore-service

CMD ["./restore-service"]