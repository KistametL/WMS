# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/api

# Stage 2: Run
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8000

CMD ["./server"]
