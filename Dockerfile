# Stage 1
FROM golang:1.16.3-alpine3.13 as builder

WORKDIR /app
COPY . .

RUN apk add --no-cache curl
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/surf

# Stage 2
FROM alpine:3.14

COPY --from=builder /app/bin/surf /surf
COPY --from=builder /usr/local/bin/yt-dlp /usr/local/bin/yt-dlp
RUN chmod a+rx /usr/local/bin/yt-dlp
RUN apk add --no-cache ffmpeg

CMD ["/surf"]
