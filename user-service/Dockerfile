FROM eclipse-temurin:17-jdk-alpine AS build

WORKDIR /src

COPY .mvn .mvn
COPY mvnw mvnw
COPY pom.xml pom.xml
RUN sh ./mvnw -B -DskipTests dependency:go-offline

COPY src src
RUN sh ./mvnw -B -DskipTests package

FROM eclipse-temurin:17-jre-alpine

RUN addgroup -S streamforge && adduser -S streamforge -G streamforge

WORKDIR /app
COPY --from=build /src/target/user-service-0.0.1-SNAPSHOT.jar /app/user-service.jar

USER streamforge
EXPOSE 8081

ENTRYPOINT ["java", "-jar", "/app/user-service.jar"]

