FROM eclipse-temurin:17-jre-alpine

RUN addgroup -S streamforge && adduser -S streamforge -G streamforge

WORKDIR /app
COPY user-service.jar /app/user-service.jar

USER streamforge
EXPOSE 8081

ENTRYPOINT ["java", "-jar", "/app/user-service.jar"]