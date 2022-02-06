package voice

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/rs/zerolog"

	"surf/internal/log"
	"surf/internal/parse"
	"surf/internal/pretty"
	"surf/pkg/lava"
)

const (
	defaultSleep             = 1 * time.Second
	defaultInactivityTimeout = 5 * time.Minute
)

var empty = struct{}{}

type session struct {
	// Mutex synchronises access to the queue and for
	// checking if the session is closing
	mu sync.Mutex
	// State used to send messages to the text channel
	// the voice state was created from
	state *state.State
	// Queue of tracks
	queue *queue
	// skip   - chan to skip a playing song
	// abort  - chan to exit the processSignals goroutine
	skip, abort chan struct{}
	// Cancels the piping of audio to the voice state
	cancelPipe context.CancelFunc
	// Information about the session, this ctx should
	// relate to where the last Join or Play command
	// was typed
	ctx SessionContext
	// loop    - should the tracks loop
	// closing - is the session closing
	loop, closing bool
	// Client to download metadata and tracks
	lava *lava.Lava
	// The track currently playing
	np lavalink.AudioTrack
	// Specific log for this session
	log zerolog.Logger
}

func newSession(ctx SessionContext, s *state.State, lava *lava.Lava) (*session, error) {
	ss := &session{
		state:      s,
		queue:      newQueue(),
		abort:      make(chan struct{}),
		skip:       make(chan struct{}),
		cancelPipe: func() {},
		lava:       lava,
	}
	lava.EnsurePlayerExists(ctx.GID)

	go ss.processSignals()
	go ss.processVoice()
	return ss, nil
}

// Internal

func (s *session) sendTyping() {
	err := s.state.Typing(s.ctx.Text)
	if err != nil {
		s.log.Error().Err(err).Interface("channel", s.ctx.Text).Msg("failed to send typing")
	}
}

func (s *session) sendMessage(content string, embeds ...discord.Embed) {
	_, err := s.state.SendMessage(s.ctx.Text, content, embeds...)
	if err != nil {
		s.log.Error().Err(err).Str("content", content).Interface("channel", s.ctx.Text).
			Interface("embeds", embeds).Msg("failed to send message")
	}
}

func (s *session) processSignals() {
	for {
		select {
		case <-s.skip:
			s.log.Debug().Msg("skipping track")
			s.mu.Lock()
			s.cancelPipe()
			s.mu.Unlock()
		case <-s.abort:
			return
		}
	}
}

func (s *session) processVoice() {
	var sleeping time.Duration

	for {
		// Sleep so we don't abuse CPU cycles
		time.Sleep(defaultSleep)
		// Check if the bot has been inactive for too long
		sleeping += defaultSleep
		if sleeping > defaultInactivityTimeout {
			s.sendMessage("Leaving voice due to inactivity")
			s.log.Debug().Msg("leaving voice due to inactivity")
			err := s.Leave()
			if err != nil {
				s.log.Error().Err(err).Msg("failed to leave voice due to inactivity")
			}
		}

		s.mu.Lock()
		// Ensure the bot isn't shutting down
		if s.closing {
			s.mu.Unlock()
			return
		}
		// Get front of the queue
		t, err := s.queue.Pop()
		if err != nil {
			s.mu.Unlock()
			continue
		}
		s.mu.Unlock()

		// Pipe the track to the voice state
		sleeping = 0
		ctx, cancel := context.WithCancel(context.Background())
		s.cancelPipe = cancel
		s.log.Debug().Str("title", t.Info().Title()).Str("author", t.Info().Author()).Msg("playing track")
		shouldExit, err := s.pipeVoice(ctx, t)
		s.log.Debug().Str("title", t.Info().Title()).Str("author", t.Info().Author()).Msg("track done")
		if err != nil && !isSignalKilled(err) && !isClosedConn(err) {
			// Only log the error if the process wasn't killed manually by us
			// or due to the connection already being closed
			s.sendMessage(fmt.Sprintf("Error playing: %s", lava.FmtTrack(t)))
			s.log.Error().Err(err).Msg("failed to pipe track")
		}
		if shouldExit {
			// If the currently playing track did not exit because it ended,
			// e.g. because there was an exception or it's stuck, then we need
			// to leave voice to reset the websocket state
			s.sendMessage(fmt.Sprintf("Error playing: %s", lava.FmtTrack(t)))
			s.log.Debug().Msg("track did not end with lava.TrackEnd")
			err := s.Leave()
			if err != nil {
				s.log.Error().Err(err).Msg("failed to leave voice due invalid track end reason")
			}
			return
		}
		s.cancelPipe()
	}
}

func (s *session) pipeVoice(ctx context.Context, t lavalink.AudioTrack) (bool, error) {
	defer func() {
		s.np = nil
	}()

	// Tells discord we are about to send the play message
	s.sendTyping()

	// Stream the audio towards the voice state
	for {
		s.np = t
		s.sendMessage("Playing: " + lava.FmtTrack(t))
		ct, err := s.lava.Play(ctx, s.ctx.GID, t)
		if err != nil {
			return false, err
		}

		if ct != lava.TrackEnd {
			return true, nil
		}

		// Play the track again if we're looping
		if s.loop {
			continue
		}

		return false, nil
	}
}

// Commands

func (s *session) Join(ctx SessionContext) error {
	// Only perform the join if we are on a new voice channel
	if s.ctx.Voice == ctx.Voice {
		return nil
	}

	// Join the new channel
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := s.state.Gateway().Send(timeout, &gateway.UpdateVoiceStateCommand{
		GuildID:   ctx.GID,
		ChannelID: ctx.Voice,
		SelfMute:  false,
		SelfDeaf:  true,
	})
	if err != nil {
		return err
	}

	s.ctx = ctx
	s.log = log.With().Uint64("gid", uint64(ctx.GID)).Logger()

	return nil
}

func (s *session) Leave() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Signify the session is closing
	s.closing = true

	// Stop writing to the voice state
	s.cancelPipe()
	s.abort <- empty
	close(s.abort)
	close(s.skip)

	// Clear the player
	err := s.lava.Close(s.ctx.GID)
	if err != nil {
		s.log.Error().Err(err).Msg("error closing lava")
	}

	// Leave the server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = s.state.Gateway().Send(ctx, &gateway.UpdateVoiceStateCommand{
		GuildID:   s.ctx.GID,
		ChannelID: discord.ChannelID(discord.NullSnowflake),
		SelfMute:  true,
		SelfDeaf:  true,
	})
	if err != nil {
		return err
	}

	// Clear the queue and other data
	s.queue.Init()
	s.queue = nil
	s.state = nil

	return nil
}

func (s *session) Play(ctx SessionContext) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Join the voice channel if needed
	err := s.Join(ctx)
	if err != nil {
		return "", err
	}

	s.ctx = ctx // We still need to set the context

	// Retrieve the track(s)
	dlCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	tracks, err := s.lava.Query(dlCtx, ctx.FirstArg())
	if err != nil {
		return "", fmt.Errorf("error finding track from link/text: %w", err)
	}
	if len(tracks) == 0 {
		return "No tracks found", errors.New("no tracks found")
	}

	// Reply if playlist of tracks
	for _, t := range tracks {
		s.log.Debug().Str("title", t.Info().Title()).Str("author", t.Info().Author()).Msg("queued track")
		s.queue.Push(t)
	}
	return fmt.Sprintf("Queued: `%d` tracks\n", len(tracks)), nil
}

func (s *session) Pause() error {
	return s.lava.Pause(s.ctx.GID)
}

func (s *session) Resume() error {
	return s.lava.Resume(s.ctx.GID)
}

func (s *session) Skip() {
	s.skip <- empty
}

func (s *session) Loop() bool {
	s.loop = !s.loop
	return s.loop
}

func (s *session) Seek(ctx SessionContext) (time.Duration, error) {
	t, err := parse.Duration(ctx.FirstArg())
	if err != nil {
		return 0, err
	}
	return t, s.lava.Seek(s.ctx.GID, t)
}

func (s *session) Queue() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return "No items in queue", nil
	}

	var resp strings.Builder
	for i, t := range s.queue.Tracks() {
		resp.WriteString(fmt.Sprintf("%d. %s\n", i+1, fmt.Sprintf("`%s` - `%s`", t.Info().Author(), t.Info().Title())))
	}
	return resp.String(), nil
}

func (s *session) NowPlaying() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.np == nil {
		return "", errors.New("no track is currently playing")
	}
	pos, err := s.lava.Position(s.ctx.GID)
	if err != nil {
		return "", err
	}
	resp := fmt.Sprintf("`%s` by `%s` - `%s`/`%s`\n", s.np.Info().Title(), s.np.Info().Author(),
		pretty.Duration(pos), pretty.Duration(s.np.Info().Length()))
	return resp, nil
}

func (s *session) ClearQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue.Init()
}

func (s *session) Remove(i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.queue.Remove(i)
}

func (s *session) Move(i, j int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Debug().Int("from", i).Int("to", j).Msg("moving track")

	return s.queue.Move(i, j)
}

func (s *session) Shuffle() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue.Shuffle()
}

// Util

func isSignalKilled(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	sig := exitErr.Sys().(syscall.WaitStatus).Signal().String()
	return sig == "killed"
}

func isClosedConn(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}
