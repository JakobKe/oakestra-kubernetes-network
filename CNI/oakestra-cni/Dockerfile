FROM golang:alpine as builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
COPY plugin.go .
COPY types.go .


RUN go mod tidy
RUN GOOS=linux GOARCH=amd64 go build -o oakestra


FROM alpine:latest

COPY --from=builder /app/oakestra .
