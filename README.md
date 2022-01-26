# surf
Self-hosted Discord music bot

## Features
- Plays YouTube/Soundcloud/Bandcamp/Spotify/Vimeo/HTTP
- Search YouTube + YouTube Music + Soundcloud
- Queue support
- Supports Pause, Resume, Skip and Seek tracks
- Supports Move and Remove tracks in the queue

## Installation
### Build from Source
```console
$ git clone https://github.com/fiwippi/surf.git
$ cd surf
$ make build
```

### Docker
1. Clone the repository
2. Configure the `docker-compose.yml` file
3. Run `docker-compose up`

⚠️ - If you're running in Docker, you should avoid specifying any of the Lavalink `.env` variables

#### GitHub Container Registry
An official container image exists at `ghcr.io/fiwippi/surf:latest`

## Config
surf requires some environment variables:
```.dotenv
# Your Discord Bot Token
BOT_TOKEN=token 

# Lavalink Config
LAVALINK_HOST=0.0.0.0
LAVALINK_PORT=2333
LAVALINK_PASS=youshallnotpass
LAVALINK_PATH=/Lavalink.jar

# If you want to support spotify you 
# can supply these two variables
SPOTIFY_ID=id
SPOTIFY_SECRET=secret
```

## Notice
This tool is meant to be used to download CC0 licenced content, it is not supported nor recommended using it for illegal activities.

## License
`BSD-3-Clause`