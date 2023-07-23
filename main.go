package main

import (
	"os"
	"os/exec"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"surf/pkg/surf"
)

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

	// Discord config
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal().Msg("no $BOT_TOKEN given")
	}

	// Spotify config
	spotifyID := os.Getenv("SPOTIFY_ID")
	if spotifyID == "" {
		log.Warn().Msg("no $SPOTIFY_ID given - bot will not support spotify")
	}
	spotifySecret := os.Getenv("SPOTIFY_SECRET")
	if spotifySecret == "" {
		log.Warn().Msg("no $SPOTIFY_SECRET given - bot will not support spotify")
	}

	// Run surf
	if err := surf.Run(token, spotifyID, spotifySecret); err != nil {
		log.Fatal().Err(err).Msg("bot error")
	}
}
