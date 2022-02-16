package voice

import (
	"context"
	"errors"
	"fmt"
	"math"
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

var (
	empty            = struct{}{}
	ErrSessionClosed = fmt.Errorf("session is closed")
)

type session struct {
	// Mutex synchronises access to the queue and for
	// checking if the session is closing
	mu sync.RWMutex
	// State used to send messages to the text channel
	// the voice state was created from
	state *state.State
	// Queue of tracks
	queue *queue
	// skip  - chan to skip a playing song
	// abort - chan to exit the processSignals goroutine
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
	// Playing is the only operation which can block for
	// long periods of time so we keep track of it's cancel
	// func so we can cancel the operation so we can leave etc.
	playCancelFunc context.CancelFunc
	// Manager so that the session can delete itself
	// from the manager if needed
	manager *Manager
}

func newSession(ctx SessionContext, s *state.State, lava *lava.Lava, m *Manager) (*session, error) {
	ss := &session{
		state:          s,
		manager:        m,
		lava:           lava,
		queue:          newQueue(),
		abort:          make(chan struct{}),
		skip:           make(chan struct{}),
		cancelPipe:     func() {},
		playCancelFunc: func() {},
	}
	lava.EnsurePlayerExists(ctx.GID)

	go ss.processSignals()
	go ss.processVoice()
	return ss, nil
}

// Internal

func (s *session) sendTyping() {
	sstate, slog, sctx := s.state, s.log, s.ctx
	if sstate != nil {
		err := sstate.Typing(sctx.Text)
		if err != nil {
			slog.Error().Err(err).Interface("channel", sctx.Text).Msg("failed to send typing")
		}
	}
}

func (s *session) sendMessage(content string, embeds ...discord.Embed) {
	sstate, slog, sctx := s.state, s.log, s.ctx
	if sstate != nil {
		_, err := sstate.SendMessage(sctx.Text, content, embeds...)
		if err != nil {
			slog.Error().Err(err).Str("content", content).Interface("channel", sctx.Text).
				Interface("embeds", embeds).Msg("failed to send message")
		}
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
		s.log.Debug().Str("title", t.Info().Title).Str("author", t.Info().Author).Msg("playing track")
		ce, err := s.pipeVoice(ctx, t)
		s.log.Debug().Err(err).Str("title", t.Info().Title).Str("author", t.Info().Author).Interface("event_type", ce.Type).Str("reason", ce.Reason).Msg("track done")
		if err != nil && !isSignalKilled(err) && !isClosedConn(err) {
			// Only log the error if the process wasn't killed manually by us
			// or due to the connection already being closed
			s.sendMessage(fmt.Sprintf("Error playing: %s", lava.FmtTrack(t)))
			s.log.Error().Err(err).Msg("failed to pipe track")
		}
		if ce.Type != lava.TrackEnd {
			// If the currently playing track did not exit because it ended,
			// e.g. because there was an exception or it's stuck, then we need
			// to leave voice to reset the websocket state
			s.sendMessage(fmt.Sprintf("Error playing: %s", lava.FmtTrack(t)))
			s.log.Debug().Str("event_type", ce.Type.String()).Str("reason", ce.Reason).Msg("track did not end with lava.TrackEnd")
			err := s.Leave()
			if err != nil {
				s.log.Error().Err(err).Msg("failed to leave voice due invalid track end reason")
			}
			return
		}
		s.cancelPipe()
	}
}

func (s *session) pipeVoice(ctx context.Context, t lavalink.AudioTrack) (lava.CloseEvent, error) {
	defer func() {
		s.np = nil
	}()

	// Tells discord we are about to send the play message
	s.sendTyping()

	// Stream the audio towards the voice state
	for {
		s.np = t
		s.sendMessage("Playing: " + lava.FmtTrack(t))
		ce, err := s.lava.Play(ctx, s.ctx.GID, t)
		if err != nil {
			return ce, err
		}
		if ce.Type != lava.TrackEnd {
			return ce, nil
		}

		// Play the track again if we're looping
		if s.loop {
			ctx, s.cancelPipe = context.WithCancel(context.Background())
			s.log.Debug().Err(err).Str("title", t.Info().Title).Str("author", t.Info().Author).Msg("looping track")
			continue
		}

		return ce, nil
	}
}

// Commands

func (s *session) Join(ctx SessionContext) error {
	// Only perform the join if we are on a new voice channel
	if s.ctx.Voice == ctx.Voice {
		return nil
	}
	if s.closing {
		return ErrSessionClosed
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
	// Signify the session is closing
	s.closing = true
	// Stop any playing blocking the processing
	s.playCancelFunc()

	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.manager.deleteSession(s.ctx)

	// Clear the queue and other data
	s.queue.Init()
	s.queue = nil
	s.state = nil

	return nil
}

func (s *session) Play(ctx SessionContext, next bool) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return "", ErrSessionClosed
	}

	// Join the voice channel if needed
	err := s.Join(ctx)
	if err != nil {
		return "", err
	}

	s.ctx = ctx // We still need to set the context

	// Retrieve the track(s)
	dlCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	s.playCancelFunc = cancelFunc
	defer cancelFunc()
	tracks, notFound, err := s.lava.Query(dlCtx, ctx.FirstArg())
	if err != nil {
		return "", fmt.Errorf("error finding track from link/text: %w", err)
	}
	if len(tracks) == 0 {
		return "No tracks found", errors.New("no tracks found")
	}
	if dlCtx.Err() != nil {
		return "", dlCtx.Err()
	}

	// We need to check if the queue is empty before we enqueue so we can decide
	// what message to send to the user later
	queueEmpty := s.queue.Len() == 0
	playingTrack := s.np != nil

	for _, t := range tracks {
		s.log.Debug().Str("title", t.Info().Title).Str("author", t.Info().Author).Msg("queued track")
	}
	if next {
		s.queue.PushFront(tracks...)
	} else {
		s.queue.PushBack(tracks...)
	}

	// Reply if playlist of tracks
	if len(tracks) > 1 {
		if notFound > 0 {
			return fmt.Sprintf("Queued: `%d` tracks, Couldn't find `%d` tracks", len(tracks), notFound), nil
		}
		return fmt.Sprintf("Queued: `%d` tracks", len(tracks)), nil
	} else {
		if queueEmpty && !playingTrack {
			return "Queued: `1` track", nil
		}
		return fmt.Sprintf("Queued: %s", lava.FmtTrack(tracks[0])), nil
	}
}

func (s *session) Pause() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return ErrSessionClosed
	}

	return s.lava.Pause(s.ctx.GID)
}

func (s *session) Resume() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return ErrSessionClosed
	}

	return s.lava.Resume(s.ctx.GID)
}

func (s *session) Skip() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return
	}

	s.skip <- empty
}

func (s *session) Loop() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return false, ErrSessionClosed
	}

	s.loop = !s.loop
	return s.loop, nil
}

func (s *session) Seek(ctx SessionContext) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return 0, ErrSessionClosed
	}

	t, err := parse.Duration(ctx.FirstArg())
	if err != nil {
		return 0, err
	}
	return t, s.lava.Seek(s.ctx.GID, t)
}

func (s *session) Queue(page int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return "", ErrSessionClosed
	}

	if s.queue.Len() == 0 {
		return "No items in queue", nil
	}
	maxPages := math.Max(1, math.Ceil(float64(s.queue.Len())/25.0))
	if page < 1 || float64(page) > maxPages {
		return "", fmt.Errorf("invalid queue page: %d", page)
	}

	start := (page - 1) * 25
	end := (page * 25) - 1

	var total time.Duration
	var resp strings.Builder
	for i, t := range s.queue.Tracks() {
		total += lava.ParseDuration(t.Info().Length)
		if i >= start && i <= end {
			resp.WriteString(fmt.Sprintf("%d. %s\n", i+1, fmt.Sprintf("`%s` - `%s`", t.Info().Author, t.Info().Title)))
		}
	}
	resp.WriteRune('\n')
	resp.WriteString(fmt.Sprintf("Page: `%d`/`%d`, Length: `%s`", page, int(maxPages), pretty.Duration(total)))
	return resp.String(), nil
}

func (s *session) NowPlaying() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return "", ErrSessionClosed
	}

	if s.np == nil {
		return "No track currently playing", nil
	}
	pos, err := s.lava.Position(s.ctx.GID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("`%s` by `%s` - `%s`/`%s`\n", s.np.Info().Title, s.np.Info().Author,
		pretty.Duration(pos), pretty.Duration(lava.ParseDuration(s.np.Info().Length))), nil
}

func (s *session) ClearQueue() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return
	}

	s.queue.Init()
}

func (s *session) Remove(i, j int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return "", ErrSessionClosed
	}

	tracks, err := s.queue.Remove(i, j)
	if err != nil {
		return "", err
	}

	if len(tracks) > 1 {
		return fmt.Sprintf("Removed `%d` tracks", len(tracks)), nil
	}
	return fmt.Sprintf("Removed %s", lava.FmtTrack(tracks[0])), nil
}

func (s *session) Move(i, j int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return "", ErrSessionClosed
	}

	s.log.Debug().Int("from", i).Int("to", j).Msg("moving track")

	t, err := s.queue.Move(i, j)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Moved %s to position `%d`", lava.FmtTrack(t), j+1), nil
}

func (s *session) Shuffle() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closing {
		return
	}

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
