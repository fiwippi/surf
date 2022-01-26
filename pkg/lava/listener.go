package lava

import (
	"github.com/DisgoOrg/disgolink/lavalink"
)

type CloseType int

const (
	TrackEnd CloseType = iota
	TrackException
	TrackStuck
	WebsocketClosed
)

type closeListener struct {
	quit chan CloseType
}

func (cl closeListener) OnPlayerPause(p lavalink.Player) {}

func (cl closeListener) OnPlayerResume(p lavalink.Player) {}

func (cl closeListener) OnPlayerUpdate(p lavalink.Player, s lavalink.PlayerState) {}

func (cl closeListener) OnTrackStart(p lavalink.Player, t lavalink.AudioTrack) {}

func (cl closeListener) OnTrackEnd(p lavalink.Player, t lavalink.AudioTrack, endReason lavalink.AudioTrackEndReason) {
	cl.quit <- TrackEnd
}

func (cl closeListener) OnTrackException(p lavalink.Player, t lavalink.AudioTrack, e lavalink.FriendlyException) {
	cl.quit <- TrackException
}

func (cl closeListener) OnTrackStuck(p lavalink.Player, t lavalink.AudioTrack, thresholdMs int) {
	cl.quit <- TrackStuck
}

func (cl closeListener) OnWebSocketClosed(p lavalink.Player, code int, reason string, byRemote bool) {
	cl.quit <- WebsocketClosed
}
