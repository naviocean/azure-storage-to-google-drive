FROM golang:1.23-alpine
WORKDIR /src

# Copy shared module
COPY shared /src/shared

# Copy restore-service
COPY do-restore-service /src/do-restore-service

# Set workdir to restore-service
WORKDIR /src/do-restore-service

# Download dependencies and build
RUN go mod download
RUN go mod tidy
RUN go build -o /app/do-restore-service

# Switch to app directory
WORKDIR /app
# COPY restore-service/credentials.json .
# COPY restore-service/token.json .

CMD ["./do-restore-service"]