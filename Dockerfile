FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main ./cmd/server

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache python3 py3-psycopg2

COPY --from=builder /app/main .
COPY --from=builder /app/web ./web
COPY --from=builder /app/mod_tool.py .

EXPOSE 8080

CMD ["./main"]
