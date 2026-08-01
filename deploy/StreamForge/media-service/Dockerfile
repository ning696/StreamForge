FROM alpine:3.22

RUN addgroup -S streamforge && adduser -S streamforge -G streamforge \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY media-service /app/media-service
RUN chmod +x /app/media-service

USER streamforge
EXPOSE 8080

ENTRYPOINT ["/app/media-service"]