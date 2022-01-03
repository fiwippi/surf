# surf
Self-hosted Discord music bot

## Features
- Plays YouTube/Soundcloud/Bandcamp/Spotify
- Search YouTube
- Queue support
- Supports Pause, Resume, and Skip tracks
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

#### GitHub Container Registry
An official container image exists at `ghcr.io/fiwippi/surf:latest`

## Config
surf requires some environment variables:
```.dotenv
# Your Discord Bot Token
BOT_TOKEN=token 

# If you want to support spotify you 
# can supply these two variables
SPOTIFY_ID=id
SPOTIFY_SECRET=secret
```

## Notice
This tool is meant to be used to download CC0 licenced content, we do not support nor recommend using it for illegal activities.

## License
`BSD-3-Clause`