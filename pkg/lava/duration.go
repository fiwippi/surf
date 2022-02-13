package lava

import (
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
)

func ParseDuration(d lavalink.Duration) time.Duration {
	return time.Millisecond * time.Duration(d.Milliseconds())
}

func ConvertDuration(d time.Duration) lavalink.Duration {
	return lavalink.Millisecond * lavalink.Duration(d.Milliseconds())
}
