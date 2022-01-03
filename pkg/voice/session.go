package voice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	"surf/internal/log"
	"surf/internal/parse"
	"surf/internal/pretty"
	"surf/pkg/ogg"
	"surf/pkg/ytdlp"
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
	// The voice session the bot writes to
	voice *voice.Session
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
	// Decodes the ogg file into opus packets
	decoder *ogg.Decoder
	// Client to download metadata and tracks
	ytdlp *ytdlp.Client
	// The track currently playing
	np ytdlp.Track
	// Specific log for this session
	log zerolog.Logger
}

func newSession(s *state.State, ytdlp *ytdlp.Client) (*session, error) {
	v, err := voice.NewSession(s)
	if err != nil {
		return nil, err
	}

	ss := &session{
		state:   s,
		voice:   v,
		queue:   newQueue(),
		abort:   make(chan struct{}),
		skip:    make(chan struct{}),
		cancelPipe: func() {},
		decoder: ogg.NewDecoder(),
		ytdlp:   ytdlp,
	}
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

func (s *session) sendMessage(content string, embeds... discord.Embed) {
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
		s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("playing track")
		err = s.pipeVoice(ctx, t.URL, t)
		s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("track done")
		if err != nil && !isSignalKilled(err) && !isClosedConn(err) {
			// Only log the error if the process wasn't killed manually by us
			// or due to the connection already being closed
			s.sendMessage(fmt.Sprintf("Error playing: %s", t.Pretty()))
			s.log.Error().Err(err).Msg("failed to play track")
		}
		s.cancelPipe()
	}
}

func (s *session) pipeVoice(ctx context.Context, url string, t ytdlp.Track) error {
	defer func() {
		s.np = ytdlp.Track{}
	}()

	// Tells discord we are about to send the play message
	s.sendTyping()

	// Download the file
	audio, err := s.ytdlp.DownloadFile(ctx, url)
	if err != nil {
		return err
	}

	// Stream the audio towards the voice state
	for {
		s.np = t
		s.sendMessage("Playing: " + t.Pretty())
		if err := s.voice.Speaking(ctx, voicegateway.Microphone); err != nil {
			return err
		}
		if err := s.decoder.Decode(ctx, s.voice, bytes.NewReader(audio)); err != nil {
			return err
		}

		// Play the track again if we're looping
		if s.loop {
			continue
		}

		return nil
	}
}

// Commands

func (s *session) Join(ctx SessionContext) error {
	// Only perform the join if we are on a new voice channel
	if s.ctx.Voice == ctx.Voice {
		return nil
	}

	err := s.voice.JoinChannel(context.Background(), ctx.Voice, false, true)
	if err != nil {
		return err
	}

	s.ctx = ctx
	s.log = log.With().Uint64("gID", uint64(ctx.GID)).Logger()

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

	// Leave the channel
	err := s.voice.Leave(context.Background())
	if err != nil {
		return err
	}

	// Clear the queue and other data
	s.queue.Init()
	s.queue = nil
	s.voice = nil
	s.decoder = nil
	s.ytdlp = nil
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

	// Retrieve the track(s)
	dlCtx, cancel := context.WithTimeout(context.Background(), 5 * time.Minute)
	defer cancel()
	tracks, err := s.ytdlp.DownloadMetadata(dlCtx, ctx.FirstArg())
	if err != nil {
		return "", err
	}
	if len(tracks) == 0 {
		return "No tracks found", errors.New("no tracks found")
	}

	// Reply if only one track
	if len(tracks) == 1 {
		t := tracks[0]
		if t.Duration > time.Hour * 3 {
			return fmt.Sprintf("Could not queue: %s - track is above 3 hours\n", t.Pretty()), nil
		} else {
			defer s.queue.Push(t)
			return fmt.Sprintf("Queued: %s\n", t.Pretty()), nil
		}
	}

	// Reply if playlist of tracks
	failed := 0
	for _, t := range tracks {
		if t.Duration > time.Hour * 3 {
			failed++
		} else {
			s.log.Debug().Str("title", t.VideoTitle).Str("url", t.URL).Msg("queued track")
			s.queue.Push(t)
		}
	}
	if failed > 0 {
		return fmt.Sprintf("Queued: `%d` tracks - `%d` failed\n", len(tracks), failed), nil
	}
	return fmt.Sprintf("Queued: `%d` tracks\n", len(tracks)), nil
}

func (s *session) Pause() {
	s.decoder.Pause()
}

func (s *session) Resume() {
	s.decoder.Resume()
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
	return t, s.decoder.Seek(t)
}

func (s *session) Queue() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return "No items in queue", nil
	}

	var resp strings.Builder
	for i, t  := range s.queue.Tracks() {
		resp.WriteString(fmt.Sprintf("%d. %s\n", i + 1, t.Pretty()))
	}
	return resp.String(), nil
}

func (s *session) NowPlaying() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.np == (ytdlp.Track{}) {
		return "", errors.New("no track is currently playing")
	}
	resp := fmt.Sprintf("`%s` by `%s` - `%s`/`%s`\n", s.np.VideoTitle, s.np.Uploader,
		pretty.Duration(s.decoder.Time), pretty.Duration(s.np.Duration))
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

	return s.queue.Move(i, j)
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