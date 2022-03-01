package voice

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
	"github.com/DisgoOrg/snowflake"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"

	"surf/pkg/lava"
)

var ErrNotSameVoiceChannel = errors.New("user is not in same voice channel as bot")

type Manager struct {
	mu    sync.Mutex
	state *state.State
	lava  *lava.Lava
	voice map[discord.GuildID]*session
}

func NewManager(s *state.State, conf lava.Config) (*Manager, error) {
	voice.AddIntents(s)

	l, err := lava.NewLava(conf)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		lava:  l,
		state: s,
		voice: make(map[discord.GuildID]*session),
	}

	s.AddHandler(func(e *gateway.VoiceStateUpdateEvent) {
		// Send the update to lavalink
		chID := snowflake.Snowflake(e.ChannelID.String())
		l.VoiceStateUpdate(lavalink.VoiceStateUpdate{
			GuildID:   snowflake.Snowflake(e.GuildID.String()),
			ChannelID: &chID,
			SessionID: e.SessionID,
		})

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

		// Count is the number of users in the voice channel who aren't the bot
		count := 0
		for _, st := range states {
			if e.ChannelID == st.ChannelID && e.UserID != st.UserID {
				count += 1
			}
		}

		ss.log.Debug().Int("count", count).Msg("voice state update")

		if count == 0 {
			now := time.Now()
			ss.lastZero = &now
		} else {
			ss.lastZero = nil
		}
	})
	s.AddHandler(func(e *gateway.VoiceServerUpdateEvent) {
		l.VoiceServerUpdate(lavalink.VoiceServerUpdate{
			Token:    e.Token,
			GuildID:  snowflake.Snowflake(e.GuildID.String()),
			Endpoint: &e.Endpoint,
		})
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

	return s.Pause()
}

func (m *Manager) Resume(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	return s.Resume()
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
	s, err := newSession(ctx, m.state, m.lava, m)
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

	// Play might block we we unlock the mutex to allow
	// the session to receive other commands, e.g. leave
	return s.Play(ctx, next)
}
