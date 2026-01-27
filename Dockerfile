# ---- Build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build router example
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/httpserver ./cmd/httpserver-router

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

# Binary
COPY --from=build /out/httpserver /app/httpserver

# Static assets only
COPY --from=build /app/static /app/static
EXPOSE 8080
ENTRYPOINT ["/app/httpserver"]
