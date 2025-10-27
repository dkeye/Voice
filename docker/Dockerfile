FROM golang:1.25 AS builder

WORKDIR /app
COPY .. .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o voice ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/voice .
CMD [ "./voice" ]