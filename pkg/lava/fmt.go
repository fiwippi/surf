package lava

import (
	"fmt"

	"github.com/DisgoOrg/disgolink/lavalink"
)

func FmtTrack(t lavalink.AudioTrack) string {
	return fmt.Sprintf("`%s` - `%s`", t.Info().Author, t.Info().Title)
}
