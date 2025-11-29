FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o capture-node ./cmd/capture-node

FROM alpine:3.19

RUN apk --no-cache add ca-certificates docker-cli

WORKDIR /app

COPY --from=builder /app/capture-node .

EXPOSE 8080

CMD ["./capture-node"]
