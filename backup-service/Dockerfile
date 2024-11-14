FROM golang:1.23-alpine


WORKDIR /src

# Copy shared module
COPY shared /src/shared

# Copy backup-service
COPY backup-service /src/backup-service

# Set workdir to backup-service
WORKDIR /src/backup-service

# Download dependencies and build
RUN go mod download
RUN go mod tidy
RUN go build -o /app/backup-service

# Switch to app directory
WORKDIR /app
# COPY backup-service/credentials.json .
# COPY backup-service/token.json .

CMD ["./backup-service"]