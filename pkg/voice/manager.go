package voice

import (
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/voice"

	"surf/pkg/ytdlp"
)

type Manager struct {
	mu     sync.Mutex
	state  *state.State
	client *ytdlp.Client
	voice  map[discord.GuildID]*session
}

func NewManager(s *state.State) *Manager {
	voice.AddIntents(s)

	return &Manager{
		state:  s,
		client: ytdlp.NewClient(),
		voice:  make(map[discord.GuildID]*session),
	}
}

// Public

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
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.joinVoice(ctx, false)
	if err != nil {
		return "", err
	}

	resp, err := s.Play(ctx)
	if err != nil {
		return "", err
	}

	return resp, nil
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
	return s.Loop(), nil
}

func (m *Manager) Queue(ctx SessionContext) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return "", err
	}
	return s.Queue()
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

func (m *Manager) Remove(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	i, err := strconv.Atoi(ctx.FirstArg())
	if err != nil {
		return err
	}
	return s.Remove(i - 1)
}

func (m *Manager) Move(ctx SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, err := m.getSession(ctx)
	if err != nil {
		return err
	}

	i, err := strconv.Atoi(ctx.FirstArg())
	if err != nil {
		return err
	}
	j, err := strconv.Atoi(ctx.SecondArg())
	if err != nil {
		return err
	}
	return s.Move(i - 1, j - 1)
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
	s, err := newSession(m.state, m.client)
	if err != nil {
		return nil, err
	}
	m.voice[ctx.GID] = s
	return s, nil
}

func (m *Manager) deleteSession(ctx SessionContext) {
	delete(m.voice, ctx.GID)
}