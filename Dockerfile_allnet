###################
# Builder stage   #
###################
FROM golang:1.21-alpine AS builder

ARG VERSION

WORKDIR /src

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Grab deps first for better cache
COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build the application for Alpine
RUN CGO_ENABLED=1 go build -ldflags="-X 'github.com/NaeuralEdgeProtocol/ratio1-backend/config.BackendVersion=${VERSION}'" -o ratio1-backend ./cmd

###################
# Runtime image   #
###################
FROM alpine:latest
ARG VERSION

LABEL org.opencontainers.image.version=$VERSION

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary
COPY --from=builder /src/ratio1-backend /app/ratio1-backend

# Copy other necessary files
COPY templates/html /app/templates/html
COPY config /app/config
COPY cmd /app/cmd

# Make executable
RUN chmod +x /app/ratio1-backend

USER 1000:1000

ENTRYPOINT ["/app/ratio1-backend"]
