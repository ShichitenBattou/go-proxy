FROM golang:1.26.0-trixie AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM debian:trixie

WORKDIR /app

COPY --from=builder /app/main .

ENTRYPOINT [ "./main" ]