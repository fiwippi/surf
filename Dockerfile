# 1
FROM golang:1.17-alpine as builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/surf

# 2
FROM openjdk:13-alpine

COPY --from=builder /app/Lavalink.jar /opt/Lavalink/Lavalink.jar
COPY --from=builder /app/application.yml /application.yml
COPY --from=builder /app/bin/surf /surf

ENV LAVALINK_HOST="0.0.0.0"
ENV LAVALINK_PORT="2333"
ENV LAVALINK_PASS="lava"
ENV LAVALINK_PATH="/opt/Lavalink/Lavalink.jar"

ENTRYPOINT ["/surf"]