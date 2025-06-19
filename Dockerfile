FROM golang:1.24-alpine as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/app ./app
EXPOSE 8080
CMD ["./app"]
