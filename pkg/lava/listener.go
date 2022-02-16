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
	Reason string // Short error to send to the user
	Error  string // Long error to be used by the program
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
		Error:  string(endReason),
		Reason: string(endReason),
	}
}

func (cl closeListener) OnTrackException(p lavalink.Player, t lavalink.AudioTrack, e lavalink.FriendlyException) {
	cl.quit <- CloseEvent{
		Type:   TrackException,
		Error:  e.Error(),
		Reason: e.Error(),
	}
}

func (cl closeListener) OnTrackStuck(p lavalink.Player, t lavalink.AudioTrack, thresholdMs lavalink.Duration) {
	cl.quit <- CloseEvent{
		Type:   TrackStuck,
		Error:  fmt.Sprintf("threshold ms: %s", thresholdMs.String()),
		Reason: "Track stuck",
	}
}

func (cl closeListener) OnWebSocketClosed(p lavalink.Player, code int, reason string, byRemote bool) {
	cl.quit <- CloseEvent{
		Type:   WebsocketClosed,
		Error:  fmt.Sprintf("code: %d reason: %s byRemote: %v", code, reason, byRemote),
		Reason: reason,
	}
}
