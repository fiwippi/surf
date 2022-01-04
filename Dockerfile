# 1
FROM golang:1.16-alpine as builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/surf

# 2
FROM python:alpine

RUN apk add --update --no-cache --virtual .build-deps gcc musl-dev && \
    pip install --upgrade pip && \
    pip install pycrypto yt-dlp && \
    rm -rf ~/.cache/pip && \
    apk del .build-deps && \
    apk add ffmpeg

COPY --from=builder /app/bin/surf /surf

CMD ["/surf"]