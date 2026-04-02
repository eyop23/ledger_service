# Stage 1 — builder: compile the binary + install goose
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 2 — runtime: minimal image, no Go toolchain
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /server .
COPY --from=builder /go/bin/goose /usr/local/bin/goose
COPY migrations/ ./migrations/
EXPOSE 8084
CMD ["/app/server"]
