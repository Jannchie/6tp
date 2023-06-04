FROM golang:1.19-alpine AS builder

WORKDIR /
COPY go.mod .
RUN go mod download
COPY . .
RUN go build -o /6tp .

FROM alpine:latest
WORKDIR /
COPY --from=builder /6tp /6tp

ENTRYPOINT ["/6tp"]