FROM golang:1.23-alpine
WORKDIR /src

# Copy shared module
COPY shared /src/shared

# Copy restore-service
COPY restore-service /src/restore-service

# Set workdir to restore-service
WORKDIR /src/restore-service

# Download dependencies and build
RUN go mod download
RUN go mod tidy
RUN go build -o /app/restore-service

# Switch to app directory
WORKDIR /app
# COPY restore-service/credentials.json .
# COPY restore-service/token.json .

CMD ["./restore-service"]