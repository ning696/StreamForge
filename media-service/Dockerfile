FROM golang:1.25.6-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/media-service ./cmd/media-service

FROM alpine:3.22

RUN addgroup -S streamforge && adduser -S streamforge -G streamforge

WORKDIR /app
COPY --from=build /out/media-service /app/media-service

USER streamforge
EXPOSE 8080

ENTRYPOINT ["/app/media-service"]

