FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vonix-api ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/vonix-api /vonix-api
EXPOSE 8080
ENTRYPOINT ["/vonix-api"]
