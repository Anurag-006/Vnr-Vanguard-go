FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 ensures a static binary that runs anywhere
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vanguard ./cmd/vanguard/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/vanguard .

COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./vanguard"]