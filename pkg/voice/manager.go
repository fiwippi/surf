package voice

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"

	ytdlp "surf/pkg/yt-dlp"
)

var ErrNotSameVoiceChannel = errors.New("user is not in same voice channel as bot")

type Manager struct {
	mu    sync.Mutex
	state *state.State
	yt    *ytdlp.Client
	voice map[discord.GuildID]*session
}

func NewManager(s *state.State, spotifyID, spotifySecrets string) (*Manager, error) {
	voice.AddIntents(s)

	m := &Manager{
		state: s,
		yt:    ytdlp.NewClient(spotifyID, spotifySecrets),
		voice: make(map[discord.GuildID]*session),
	}

	me, err := s.Me()
	if err != nil {
		return nil, err
	}

	s.AddHandler(func(e *gateway.VoiceStateUpdateEvent) {
		// Update the sessions last timestamp when it had
		// zero users in the vc if applicable
		ss, ok := m.voice[e.GuildID]
		if !ok || (ss != nil && ss.closing) {
			return
		}

		states, err := s.VoiceStates(e.GuildID)
		if err != nil {
			ss.log.Error().Err(err).Msg("could not get voice states")
			return
		}

		// The channel ID may be null, if this happens then manually retrieve it
		// from the voice states
		if e.ChannelID == discord.NullChannelID {
			for _, st := range states {
				if st.UserID == me.ID {
					e.ChannelID = st.ChannelID
				}
			}
		}
		// If the channel ID is still null then exit
		if e.ChannelID == discord.NullChannelID {
			ss.log.Error().Msg("null channel id for voice state update")
			return
		}

		// Count is the number of users in the voice channel including the bot
		count := 0
		for _, st := range states {
			if e.ChannelID == st.ChannelID {
				count += 1
			}
		}

		ss.log.Debug().Int("guild_count", len(states)).Int("channel_count", count).Msg("voice state update")

		// 1 means only the bot is in the channel
		if count == 1 {
			now := time.Now()
			ss.lastZero = &now
		} else {
			ss.lastZero = nil
		}
	})

	return m, nil
}

// Public

func (m *Manager) SameVoiceChannel(ctx SessionContext) bool {
	s, ok := m.voice[ctx.GID]
	if !ok || s == nil {
		// If the session doesn't exist treat it like the
		// bot is in the same voice channel as the user
		return true
	}
	return ctx.VID == s.ctx.VID
}

func (m *Manager) JoinVoice(ctx SessionContext) error {
	_, err := m.joinVoice(ctx, true)
	return err
}

func (m *Manager) LeaveVoice(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if a voice state already exists
	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}
	// Now we tell the session to leave the voice channel
	err = s.Leave()
	if err != nil {
		return err
	}
	// Remove it from the map, we replace it with a newly created session once we join again
	m.deleteSession(ctx)

	return nil
}

func (m *Manager) Play(ctx SessionContext) (string, error) {
	return m.play(ctx, false)
}

func (m *Manager) PlayNext(ctx SessionContext) (string, error) {
	return m.play(ctx, true)
}

func (m *Manager) Skip(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}
	s.Skip()
	return nil
}

func (m *Manager) Pause(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	s.Pause()
	return nil
}

func (m *Manager) Resume(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	s.Resume()
	return nil
}

func (m *Manager) Seek(ctx SessionContext) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return 0, err
	}
	return s.Seek(ctx)
}

func (m *Manager) Loop(ctx SessionContext) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return false, err
	}
	return s.Loop()
}

func (m *Manager) Queue(ctx SessionContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var page = 1
	var err error
	if ctx.HasFirstArg() {
		page, err = strconv.Atoi(ctx.FirstArg())
		if err != nil {
			return "", err
		}
	}

	s, err := m.getSession(ctx)
	if err != nil {
		return "", err
	}

	return s.Queue(page)
}

func (m *Manager) NowPlaying(ctx SessionContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return "", err
	}
	return s.NowPlaying()
}

func (m *Manager) Clear(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}
	s.ClearQueue()
	return nil
}

func (m *Manager) Remove(ctx SessionContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return "", err
	}

	i, err := strconv.Atoi(ctx.FirstArg())
	if err != nil {
		return "", err
	}

	j := i
	if ctx.HasSecondArg() {
		var err error
		j, err = strconv.Atoi(ctx.SecondArg())
		if err != nil {
			return "", err
		}
	}

	return s.Remove(i-1, j-1)
}

func (m *Manager) Move(ctx SessionContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return "", err
	}

	i, err := strconv.Atoi(ctx.FirstArg())
	if err != nil {
		return "", err
	}
	j, err := strconv.Atoi(ctx.SecondArg())
	if err != nil {
		return "", err
	}
	return s.Move(i-1, j-1)
}

func (m *Manager) Shuffle(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	s.Shuffle()
	return nil
}

// Private

func (m *Manager) joinVoice(ctx SessionContext, lock bool) (*session, error) {
	if lock {
		m.mu.Lock()
		defer m.mu.Unlock()
	}

	// Check if a voice session already exists
	var err error
	var s *session
	s, err = m.getSession(ctx)
	if err != nil {
		if err == ErrNotSameVoiceChannel {
			return nil, err
		}
		// If a session doesn't exist we create one
		s, err = m.createSession(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Now we tell the session to join the voice channel
	err = s.Join(ctx)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (m *Manager) getSession(ctx SessionContext) (*session, error) {
	s, ok := m.voice[ctx.GID]
	if s != nil && ctx.VID != s.ctx.VID {
		return nil, ErrNotSameVoiceChannel
	}
	if s != nil && s.closing {
		m.deleteSession(ctx)
		ok = false
	}
	if !ok {
		return nil, errors.New("no session exists")
	}
	return s, nil
}

func (m *Manager) createSession(ctx SessionContext) (*session, error) {
	s, err := newSession(m.state, m.yt, m)
	if err != nil {
		return nil, err
	}
	m.voice[ctx.GID] = s
	return s, nil
}

func (m *Manager) deleteSession(ctx SessionContext) {
	delete(m.voice, ctx.GID)
}

func (m *Manager) play(ctx SessionContext, next bool) (string, error) {
	m.mu.Lock()
	s, err := m.joinVoice(ctx, false)
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	m.mu.Unlock()

	// Play might block, so we unlock the mutex to allow
	// the session to receive other commands, e.g. leave
	return s.Play(ctx, next)
}
