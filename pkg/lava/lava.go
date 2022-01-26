package lava

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
	"github.com/DisgoOrg/snowflake"
	"github.com/diamondburned/arikawa/v3/discord"

	"surf/internal/log"
)

var ErrNoPlayer = errors.New("no player exists")
var ErrNoRestClient = errors.New("no rest client available")

type Lava struct {
	l lavalink.Lavalink
	s *spotifyClient
}

type search struct {
	T    lavalink.AudioTrack
	Diff time.Duration
}

func NewLava(conf Config) (*Lava, error) {
	l := lavalink.New(
		lavalink.WithUserID(snowflake.Snowflake(conf.AppID.String())),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	node, err := l.AddNode(ctx, lavalink.NodeConfig{
		Name:     "surf",
		Host:     conf.Host,
		Port:     conf.Port,
		Password: conf.Pass,
		Secure:   false,
	})
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node could not be added")
	}

	return &Lava{
		l: l,
		s: newSpotifyClient(conf.SpotifyID, conf.SpotifySecret),
	}, nil
}

func (l *Lava) search(searchType lavalink.SearchType, query string) ([]lavalink.AudioTrack, error) {
	rc := l.l.BestRestClient()
	if rc == nil {
		return nil, ErrNoRestClient
	}

	resp, err := rc.LoadItem(searchType.Apply(query))
	if err != nil {
		return nil, fmt.Errorf("failed to load %s track: %w", searchType, err)
	}
	if resp.LoadType != lavalink.LoadTypeSearchResult {
		return nil, fmt.Errorf("search result not returned for query")
	}
	if len(resp.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks returned for track: %s", query)
	}
	return resp.Tracks, nil
}

func (l *Lava) searchComplex(st spotifyTrack) (lavalink.AudioTrack, error) {
	// First we perform the search
	tracks, err := l.search(lavalink.SearchTypeYoutubeMusic, fmt.Sprintf("%s %s", st.Artist, st.Title))
	if err != nil {
		return nil, err
	}

	// Now filter the searches
	filtered := make([]search, 0)
	for _, track := range tracks {
		diff := st.Duration - track.Info().Length()
		if diff < 0 {
			diff *= -1
		}

		containsArtist := strings.Contains(track.Info().Author(), st.Artist)
		containsTitle := strings.Contains(track.Info().Title(), st.Title)
		if containsArtist && containsTitle {
			filtered = append(filtered, search{
				T:    track,
				Diff: diff,
			})
		}
	}
	if len(filtered) == 0 {
		return tracks[0], nil
	}

	// If we managed to find some tracks
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Diff < filtered[j].Diff
	})
	return filtered[0].T, nil
}

func (l *Lava) link(link string) ([]lavalink.AudioTrack, error) {
	rc := l.l.BestRestClient()
	if rc == nil {
		return nil, ErrNoRestClient
	}

	resp, err := rc.LoadItem(link)
	if err != nil {
		return nil, fmt.Errorf("failed to load item: %w", err)
	}
	if resp.Exception != nil {
		return nil, fmt.Errorf("resp has friendly exception: %w", resp.Exception)
	}
	if len(resp.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks returned for item")
	}

	i := resp.PlaylistInfo.SelectedTrack
	if i != -1 {
		return []lavalink.AudioTrack{resp.Tracks[i]}, nil
	}
	return resp.Tracks, nil
}

func (l *Lava) Query(ctx context.Context, searchType lavalink.SearchType, text string) ([]lavalink.AudioTrack, error) {
	_, err := url.ParseRequestURI(text)
	if err != nil {
		// If we have a search term
		t, err := l.search(searchType, text)
		if err != nil {
			return nil, err
		}
		return []lavalink.AudioTrack{t[0]}, nil
	} else {
		// If we have a URL we first check if it's a spotify url
		if !strings.Contains(text, "spotify.com") {
			tracks, err := l.link(text)
			if err != nil {
				return nil, err
			}
			return tracks, nil
		}

		// If it is a spotify URL we attempt to download the tracks
		if l.s == nil {
			return nil, errors.New("spotify is unsupported")
		}
		queries, err := l.s.Download(ctx, text)
		if err != nil {
			return nil, err
		}

		// Now we search for the tracks on youtube using spotify metadata
		tracks := make([]lavalink.AudioTrack, 0)
		for _, q := range queries {
			t, err := l.searchComplex(q)
			if err != nil {
				log.Error().Err(err).Interface("track", t).Msg("failed to search for spotify track")
			} else {
				tracks = append(tracks, t)
			}
		}

		if len(tracks) == 0 {
			return nil, errors.New("no tracks found")
		}
		return tracks, nil
	}
}

func (l *Lava) EnsurePlayerExists(guildID discord.GuildID) {
	l.l.Player(snowflake.Snowflake(guildID.String()))
}

func (l *Lava) Play(ctx context.Context, guildID discord.GuildID, t lavalink.AudioTrack) (CloseType, error) {
	n := l.l.BestNode()
	if n == nil {
		return TrackEnd, fmt.Errorf("node doesn't exist")
	}

	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return TrackEnd, ErrNoPlayer
	}

	done := make(chan CloseType)
	listener := closeListener{quit: done}
	p.AddListener(listener)

	err := p.Play(t)
	if err != nil {
		return TrackEnd, err
	}

	ct := TrackEnd
	select {
	case <-ctx.Done():
	case <-time.After(t.Info().Length() + 30*time.Second):
	case t := <-done:
		ct = t
	}

	p.RemoveListener(listener)
	close(done)

	return ct, p.Stop()
}

func (l *Lava) Pause(guildID discord.GuildID) error {
	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return ErrNoPlayer
	}

	return p.Pause(true)
}

func (l *Lava) Resume(guildID discord.GuildID) error {
	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return ErrNoPlayer
	}

	return p.Pause(false)
}

func (l *Lava) Seek(guildID discord.GuildID, t time.Duration) error {
	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return ErrNoPlayer
	}

	return p.Seek(t)
}

func (l *Lava) Position(guildID discord.GuildID) (time.Duration, error) {
	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return 0, ErrNoPlayer
	}

	return p.Position(), nil
}

func (l *Lava) Close(guildID discord.GuildID) error {
	p := l.l.ExistingPlayer(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return nil
	}

	if p.Node().Status() != lavalink.Connected {
		return nil
	}
	return p.Destroy()
}

func (l *Lava) VoiceServerUpdate(vsu lavalink.VoiceServerUpdate) {
	l.l.VoiceServerUpdate(vsu)
}

func (l *Lava) VoiceStateUpdate(vsu lavalink.VoiceStateUpdate) {
	l.l.VoiceStateUpdate(vsu)
}
