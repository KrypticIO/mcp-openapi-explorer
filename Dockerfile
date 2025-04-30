FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-openapi-explorer

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/mcp-openapi-explorer .

# Create specs directory
RUN mkdir -p /app/specs

# Set up environment variables
ENV GITHUB_TOKEN=""

# Default to serve command
ENTRYPOINT ["./mcp-openapi-explorer"]
CMD ["serve"] 