FROM golang:1.25 AS builder

WORKDIR /app
COPY ../go.mod ../go.sum ./
RUN go mod download
COPY .. .
RUN CGO_ENABLED=0 GOOS=linux go build -o voice ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/voice .
COPY --from=builder /app/web ./web
COPY --from=builder /app/config ./config
CMD [ "./voice" ]