package main

import (
	"os"

	"github.com/joho/godotenv"

	"surf/internal/log"
	"surf/pkg/lava"
	"surf/pkg/surf"
)

func main() {
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

	// Lavalink config
	lavalinkHost := os.Getenv("LAVALINK_HOST")
	if lavalinkHost == "" {
		log.Fatal().Msg("no $LAVALINK_HOST given")
	}
	lavalinkPort := os.Getenv("LAVALINK_PORT")
	if lavalinkPort == "" {
		log.Fatal().Msg("no $LAVALINK_PORT given")
	}
	lavalinkPass := os.Getenv("LAVALINK_PASS")
	if lavalinkPass == "" {
		log.Fatal().Msg("no $LAVALINK_PASS given")
	}
	lavalinkPath := os.Getenv("LAVALINK_PATH")
	if lavalinkPath == "" {
		log.Fatal().Msg("no $LAVALINK_PATH given")
	}

	// Ensure Lavalink.jar file exists
	if _, err := os.Stat(lavalinkPath); os.IsNotExist(err) {
		log.Fatal().Err(err).Str("path", lavalinkPath).Msg("Lavalink.jar file does not exist")
	}

	conf := lava.Config{
		Host:          lavalinkHost,
		Port:          lavalinkPort,
		Pass:          lavalinkPass,
		SpotifyID:     spotifyID,
		SpotifySecret: spotifySecret,
	}
	if err := surf.Run(token, conf, lavalinkPath); err != nil {
		log.Fatal().Err(err).Msg("bot error")
	}
}
