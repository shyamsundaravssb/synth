FROM golang:alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /synth-server ./cmd/synth-server

# Use alpine as base image for the final stage because we need curl for docker-compose healthchecks
FROM alpine:latest
RUN apk --no-cache add ca-certificates curl

WORKDIR /app
COPY --from=builder /synth-server /app/synth-server

ENV PORT=8080
EXPOSE 8080

CMD ["/app/synth-server"]
