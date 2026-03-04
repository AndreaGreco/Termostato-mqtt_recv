FROM golang:1.25

WORKDIR /app

# Install PostgreSQL client for pg_isready
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

# Copy go modules files
COPY config.yaml go.mod go.sum ./
RUN go mod download

# Install tern migration tool
RUN go install github.com/jackc/tern/v2@latest

# Copy application code
COPY ./ /app

# Build application
RUN go build -o /app/mqtt_client

# Make entrypoint script executable
RUN chmod +x /app/entrypoint.sh

# Use entrypoint script
ENTRYPOINT ["/app/entrypoint.sh"]
