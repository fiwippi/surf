# surf
Self-hosted Discord music bot

## Features
- Plays YouTube/Soundcloud/Spotify/Bandcamp
- Queue support: Play, Pause, Resume, Now Playing, Skip, Seek, Move, Remove, Clear, Shuffle, Loop

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

#### GitHub Container Registry
An official container image exists at `ghcr.io/fiwippi/surf:latest`

## Config
surf requires some environment variables:
```dotenv
BOT_TOKEN=token       # Your Discord Bot Token
SPOTIFY_ID=id         # Your Spotify Client ID
SPOTIFY_SECRET=secret # Your Spotify Client Secret
```

## Notice
This tool is meant to be used to download CC0 licenced content, it is not supported nor recommended using it for illegal activities.

## License
`BSD-3-Clause`
