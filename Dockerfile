# syntax=docker/dockerfile:1

# Build the frontend.
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build the server.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ledger ./cmd/ledger

# Runtime image: static binary plus the built frontend.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ledger /app/ledger
COPY --from=web /web/dist /app/web/dist
ENV STATIC_DIR=/app/web/dist
EXPOSE 8080
CMD ["/app/ledger", "serve"]
