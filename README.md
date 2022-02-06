# surf
Self-hosted Discord music bot

## Features
- Plays YouTube/Soundcloud/Bandcamp/Spotify/Vimeo/HTTP
- Queue support:
    - Now Playing
    - Pause
    - Resume
    - Skip
    - Seek
    - Move
    - Remove
    - Clear
    - Shuffle
    - Loop

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
```dotenv
# Your Discord Bot Token
BOT_TOKEN=token 

# For Spotify supply these values
SPOTIFY_ID=id
SPOTIFY_SECRET=secret
```

⚠️ - Within Docker surf runs its own Lavalink instance, but if you're running your own Lavalink instance you should supply the following `.env` variables. If you'd like surf to handle running the Lavalink `.jar` as well then you can also specify its absolute filepath.
```dotenv
# Required
LAVALINK_HOST=0.0.0.0
LAVALINK_PORT=2333
LAVALINK_PASS=youshallnotpass

# Optional
LAVALINK_PATH=/Lavalink.jar
```


## Notice
This tool is meant to be used to download CC0 licenced content, it is not supported nor recommended using it for illegal activities.

## License
`BSD-3-Clause`