package voice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"surf/internal/parse"
	"surf/internal/pretty"
	"surf/pkg/ogg"
	ytdlp "surf/pkg/yt-dlp"
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
	// The voice session the bot writes to
	voice *voice.Session
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
	// Decodes the ogg file into opus packets
	decoder *ogg.Decoder
	// Client to download metadata and tracks
	yt *ytdlp.Client
	// The track currently playing
	np *ytdlp.Track
	// Specific log for this session
	log zerolog.Logger
	// Playing is the only operation which can block for
	// long periods of time so we keep track of it's cancel
	// func so we can cancel the operation so we can leave etc.
	playCancelFunc context.CancelFunc
	// Manager so that the session can delete itself
	// from the manager if needed
	manager *Manager
	// Ensures the leaving only happens once
	once sync.Once
	// Last time since there were zero users in a voice channel
	// If this was more than 5 minutes ago then the bot leaves due to inactivity
	lastZero *time.Time
}

func newSession(s *state.State, yt *ytdlp.Client, m *Manager) (*session, error) {
	v, err := voice.NewSession(s)
	if err != nil {
		return nil, err
	}

	ss := &session{
		state:          s,
		manager:        m,
		voice:          v,
		yt:             yt,
		queue:          newQueue(yt),
		decoder:        ogg.NewDecoder(),
		abort:          make(chan struct{}),
		skip:           make(chan struct{}),
		cancelPipe:     func() {},
		playCancelFunc: func() {},
	}

	go ss.processSignals()
	go ss.processVoice()
	go ss.processEmptyVC()
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

func (s *session) leaveDueToInactivity() {
	s.sendMessage("Leaving voice due to inactivity")
	s.log.Debug().Msg("leaving voice due to inactivity")
	err := s.Leave()
	if err != nil {
		s.log.Error().Err(err).Msg("failed to leave voice due to inactivity")
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

func (s *session) processEmptyVC() {
	for {
		time.Sleep(15 * time.Second)

		if s.closing {
			return
		} else if s.lastZero != nil && time.Since(*s.lastZero) > defaultInactivityTimeout {
			log.Debug().Str("lastZero", (*s.lastZero).String()).Msg("inactivity time")
			s.leaveDueToInactivity()
			return
		}
	}
}

func (s *session) processVoice() {
	var sleeping time.Duration // Time since the bot is idle (not playing a track)

	for {
		// Sleep so we don't abuse CPU cycles
		time.Sleep(defaultSleep)
		// Increment the inactivity timeout
		sleeping += defaultSleep

		// Check if the bot has been inactive for too long
		a := sleeping > defaultInactivityTimeout
		b := s.lastZero != nil && time.Since(*s.lastZero) > defaultInactivityTimeout
		if a || b {
			s.leaveDueToInactivity()
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
		s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("playing track")
		err = s.pipeVoice(ctx, t)
		s.log.Debug().Err(err).Str("title", t.VideoTitle).Str("url", t.Uploader).Msg("track done")
		if err != nil && !isSignalKilled(err) && !isClosedConn(err) {
			// Only log the error if the process wasn't killed manually by us
			// or due to the connection already being closed
			s.sendMessage(fmt.Sprintf("Error playing: %s", t.Pretty()))
			s.log.Error().Err(err).Msg("failed to pipe track")
		}
		s.cancelPipe()
	}
}

func (s *session) pipeVoice(ctx context.Context, t *ytdlp.Track) error {
	defer func() {
		s.mu.RLock()
		s.np = nil
		s.mu.RUnlock()
	}()

	// Tells discord we are about to send the play message
	s.sendTyping()

	// Download the file
	audio, ok := <-t.FileChan()
	if !ok {
		return errors.New("file chan closed (shouldn't happen here)")
	}
	if audio == nil {
		return errors.New("file failed to download")
	}

	// Stream the audio towards the voice state
	for {
		s.mu.RLock()
		s.np = t
		s.mu.RUnlock()

		s.sendMessage("Playing: " + t.Pretty())
		if err := s.voice.Speaking(ctx, voicegateway.Microphone); err != nil {
			return err
		}
		if err := s.decoder.Decode(ctx, s.voice, bytes.NewReader(audio)); err != nil {
			return err
		}

		// Play the track again if we're looping
		if s.loop {
			ctx, s.cancelPipe = context.WithCancel(context.Background())
			s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("looping track")
			continue
		}

		return nil
	}
}

// Commands

func (s *session) Join(ctx SessionContext) error {
	// Only perform the join if we are on a new voice channel
	if s.ctx.VID == ctx.VID {
		return nil
	}
	if s.closing {
		return ErrSessionClosed
	}

	// Join the new channel
	err := s.voice.JoinChannel(context.Background(), ctx.VID, false, true)
	if err != nil {
		return err
	}

	s.ctx = ctx
	s.log = log.With().Str("guild", ctx.Guild).Str("channel", ctx.Voice).Logger()
	s.log.Debug().Msg("joined voice")

	return nil
}

func (s *session) Leave() error {
	// Signify the session is closing
	s.closing = true
	// Stop any playing blocking the processing
	s.playCancelFunc()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Any errors returned from the closing operation
	var leaveErr error

	// Leaving should only happen once
	s.once.Do(func() {
		// Stop writing to the voice state
		s.cancelPipe()
		s.abort <- empty
		close(s.abort)
		close(s.skip)

		// Leave the channel
		err := s.voice.Leave(context.Background())
		if err != nil {
			leaveErr = err
			return
		}

		// Clear the session, queue and other data
		s.manager.deleteSession(s.ctx)
		s.queue.Init()
		s.queue = nil
		s.state = nil
		s.manager = nil
		s.lastZero = nil

		leaveErr = nil
	})

	return leaveErr
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
	tracks, err := s.yt.DownloadMetadata(dlCtx, ctx.FirstArg())
	if err != nil {
		return "", fmt.Errorf("error finding track from link/text: %w", err)
	}
	if len(tracks) == 0 {
		return "No tracks found", errors.New("no tracks found")
	}
	if dlCtx.Err() != nil {
		return "Error Encountered...", dlCtx.Err()
	}

	// We need to check if the queue is empty before we enqueue so we can decide
	// what message to send to the user later
	queueEmpty := s.queue.Len() == 0
	playingTrack := s.np != nil

	// Anonymous function to simplify queueing
	qtrack := func(t *ytdlp.Track) {
		if next {
			s.queue.PushFront(t)
		} else {
			s.queue.PushBack(t)
		}
	}

	// Reply and queueing behaviour if only one track returned
	if len(tracks) == 1 {
		t := tracks[0]
		if t.Duration > time.Hour*3 {
			return fmt.Sprintf("Could not queue: %s - track is above 3 hours\n", t.Pretty()), nil
		} else {
			qtrack(t)
			if queueEmpty && !playingTrack {
				return "Queued: `1` track", nil
			}
			return fmt.Sprintf("Queued: %s", t.Pretty()), nil
		}
	}

	// Reply and queueing if more than one track is returned
	failed := 0
	for _, t := range tracks {
		if t.Duration > time.Hour*3 {
			failed++
		} else {
			s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("queued track")
			qtrack(t)
		}
	}
	if failed > 0 {
		return fmt.Sprintf("Queued: `%d` tracks - `%d` failed\n", len(tracks), failed), nil
	}
	return fmt.Sprintf("Queued: `%d` tracks\n", len(tracks)), nil
}

func (s *session) Pause() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.decoder.Pause()
}

func (s *session) Resume() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.decoder.Resume()
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
	return t, s.decoder.Seek(t)
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
		total += t.Duration
		if i >= start && i <= end {
			resp.WriteString(fmt.Sprintf("%d. %s\n", i+1, fmt.Sprintf("%s (%s)`",
				t.Pretty(), pretty.Duration(t.Duration))))
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

	return fmt.Sprintf("`%s` by `%s` - `%s`/`%s`\n", s.np.VideoTitle, s.np.Uploader,
		pretty.Duration(s.decoder.Time), pretty.Duration(s.np.Duration)), nil
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
	return fmt.Sprintf("Removed %s", tracks[0].Pretty()), nil
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

	return fmt.Sprintf("Moved %s to position `%d`", t.Pretty(), j+1), nil
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
