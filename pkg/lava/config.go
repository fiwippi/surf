package lava

import "github.com/diamondburned/arikawa/v3/discord"

type Config struct {
	Host          string
	Port          string
	AppID         discord.AppID
	Pass          string
	SpotifyID     string
	SpotifySecret string
}
