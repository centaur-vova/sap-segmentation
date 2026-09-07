FROM golang:1.26.0-alpine3.23 AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a \
    -installsuffix cgo \
    -ldflags="-s -w" \
    -o sap_segmentationd \
    ./cmd/sap_segmentationd

FROM alpine:3.23

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/sap_segmentationd .

RUN mkdir -p /log

CMD ["./sap_segmentationd"]