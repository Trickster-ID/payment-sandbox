# syntax=docker/dockerfile:1
FROM golang:1.26-alpine3.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/myapp ./app/cmd/

FROM scratch

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/myapp /app/myapp

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/app/myapp"]
