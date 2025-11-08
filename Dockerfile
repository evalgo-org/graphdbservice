FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags '-w -s' -o graphdbservice ./cmd

FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/graphdbservice /app/
RUN chmod +x /app/graphdbservice
ENTRYPOINT ["/app/graphdbservice"]
CMD ["service"]
