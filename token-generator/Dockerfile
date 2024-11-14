FROM golang:1.23-alpine

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod .
COPY go.sum .
RUN go mod download

# Then copy source code
COPY . .

# Build
RUN go build -o token-generator

CMD ["./token-generator"]