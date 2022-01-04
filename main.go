package main

import (
	"os"
	"os/exec"

	"github.com/joho/godotenv"

	"surf/internal/log"
	"surf/pkg/surf"
)

// TODO handle blocking for a short command whilst we are waiting
//       i.e. if we receive "queue" whilst the session is processing
//       a "play" command the interaction will fail because it will
//		 take more than 3 seconds to respond to play
// TODO download the next track in the queue so we don't have to
//       wait to download it

func mustExec(path string) {
	_, err := exec.LookPath(path)
	if err != nil {
		log.Fatal().Err(err).Msg("no " + path + " found")
	}
}

func main() {
	mustExec("yt-dlp")
	mustExec("ffmpeg")

	godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal().Msg("no $BOT_TOKEN given")
	}
	spotifyID := os.Getenv("SPOTIFY_ID")
	if spotifyID == "" {
		log.Warn().Msg("no $SPOTIFY_ID given - bot will not support spotify")
	}
	spotifySecret := os.Getenv("SPOTIFY_SECRET")
	if spotifySecret == "" {
		log.Warn().Msg("no $SPOTIFY_SECRET given - bot will not support spotify")
	}

	if err := surf.Run(token); err != nil {
		log.Fatal().Err(err).Msg("bot error")
	}
}
