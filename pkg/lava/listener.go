package lava

import (
	"fmt"

	"github.com/DisgoOrg/disgolink/lavalink"
)

type CloseType int

func (ct CloseType) String() string {
	return [...]string{"TrackEnd", "TrackException", "TrackStuck", "WebsocketClosed"}[ct]
}

const (
	TrackEnd CloseType = iota
	TrackException
	TrackStuck
	WebsocketClosed
)

type CloseEvent struct {
	Type   CloseType
	Reason string
}

type closeListener struct {
	quit chan CloseEvent
}

var _ lavalink.PlayerEventListener = (*closeListener)(nil)

func (cl closeListener) OnPlayerPause(p lavalink.Player) {}

func (cl closeListener) OnPlayerResume(p lavalink.Player) {}

func (cl closeListener) OnPlayerUpdate(p lavalink.Player, s lavalink.PlayerState) {}

func (cl closeListener) OnTrackStart(p lavalink.Player, t lavalink.AudioTrack) {}

func (cl closeListener) OnTrackEnd(p lavalink.Player, t lavalink.AudioTrack, endReason lavalink.AudioTrackEndReason) {
	cl.quit <- CloseEvent{
		Type:   TrackEnd,
		Reason: string(endReason),
	}
}

func (cl closeListener) OnTrackException(p lavalink.Player, t lavalink.AudioTrack, e lavalink.FriendlyException) {
	cl.quit <- CloseEvent{
		Type:   TrackException,
		Reason: e.Error(),
	}
}

func (cl closeListener) OnTrackStuck(p lavalink.Player, t lavalink.AudioTrack, thresholdMs lavalink.Duration) {
	cl.quit <- CloseEvent{
		Type:   TrackStuck,
		Reason: fmt.Sprintf("threshold ms: %s", thresholdMs.String()),
	}
}

func (cl closeListener) OnWebSocketClosed(p lavalink.Player, code int, reason string, byRemote bool) {
	cl.quit <- CloseEvent{
		Type:   WebsocketClosed,
		Reason: fmt.Sprintf("code: %d reason: %s byRemote: %v", code, reason, byRemote),
	}
}
