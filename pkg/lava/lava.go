package lava

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
	simpleLog "github.com/DisgoOrg/log"
	"github.com/DisgoOrg/snowflake"
	"github.com/diamondburned/arikawa/v3/discord"
	edlib "github.com/hbollon/go-edlib"
	"golang.org/x/time/rate"

	"surf/internal/log"
)

var ErrNoPlayer = errors.New("no player exists")
var ErrNoRestClient = errors.New("no rest client available")

type Lava struct {
	l  lavalink.Lavalink
	s  *spotifyClient
	rl *rate.Limiter
}

type search struct {
	T          lavalink.AudioTrack
	TimeDiff   time.Duration
	StringDiff int
}

type orderedTrack struct {
	order int
	t     lavalink.AudioTrack
}

func NewLava(conf Config) (*Lava, error) {
	custL := simpleLog.Default()
	custL.SetLevel(simpleLog.LevelFatal)
	l := lavalink.New(
		lavalink.WithLogger(custL),
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
		l:  l,
		s:  newSpotifyClient(conf.SpotifyID, conf.SpotifySecret),
		rl: rate.NewLimiter(50, 1),
	}, nil
}

func (l *Lava) search(ctx context.Context, searchType lavalink.SearchType, query string) ([]lavalink.AudioTrack, error) {
	rc := l.l.BestRestClient()
	if rc == nil {
		return nil, ErrNoRestClient
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	resp, err := rc.LoadItem(ctx, searchType.Apply(query))
	if err != nil {
		return nil, fmt.Errorf("failed to load %s track: %w", searchType, err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if resp.LoadType != lavalink.LoadTypeSearchResult {
		return nil, fmt.Errorf("search result not returned for query")
	}
	if len(resp.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks returned for track: %s", query)
	}
	parsed, err := l.parseTracks(resp.Tracks...)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (l *Lava) searchSpotifyFiltered(ctx context.Context, st spotifyTrack, searchType lavalink.SearchType, useArtist, useSimilarity bool) (lavalink.AudioTrack, error) {
	searchTerm := fmt.Sprintf("%s - %s", st.Artist, st.Title)
	tracks, err := l.search(ctx, searchType, searchTerm)
	if err != nil {
		return nil, err
	}

	// Now filter the searches
	filtered := make([]search, 0)
	for _, track := range tracks {
		diff := st.Duration - ParseDuration(track.Info().Length)
		if diff < 0 {
			diff *= -1
		}

		// If the found track's author is the spotify artist
		lowerArtist := strings.ToLower(st.Artist)
		lowerArtistSt := strings.ToLower(track.Info().Author)
		containsArtist := strings.Contains(lowerArtist, lowerArtistSt)
		// If the found track contains the spotify track's title
		// For contains the title we also check the case for split hyphens
		lowerTitle := strings.ToLower(track.Info().Title)
		lowerTitleSt := strings.ToLower(st.Title)
		titleA := strings.Contains(lowerTitle, lowerTitleSt)
		titleB := strings.Contains(lowerTitle, strings.ReplaceAll(lowerTitleSt, "-", " "))
		containsTitle := titleA || titleB
		// How similar both track names are, takes into account
		// (youtube) videos where the artist name is also in the
		// video
		simArtist := edlib.DamerauLevenshteinDistance(lowerArtistSt, lowerArtist)
		simA := edlib.DamerauLevenshteinDistance(st.Title, track.Info().Title) + simArtist
		simB := edlib.DamerauLevenshteinDistance(searchTerm, track.Info().Title) + simArtist
		similarity := simA
		if simB < simA {
			similarity = simB
		}
		if (containsArtist && containsTitle) || (!useArtist && containsTitle) {
			filtered = append(filtered, search{
				T:          track,
				TimeDiff:   diff,
				StringDiff: similarity,
			})
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("could not find track")
	}

	if !useSimilarity {
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].TimeDiff < filtered[j].TimeDiff
		})
	} else {
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].StringDiff < filtered[j].StringDiff
		})
	}

	return filtered[0].T, nil
}

func (l *Lava) searchSpotify(ctx context.Context, st spotifyTrack) (lavalink.AudioTrack, error) {
	track, err := l.searchSpotifyFiltered(ctx, st, lavalink.SearchTypeYoutubeMusic, true, false)
	if err == nil {
		return track, nil
	}
	track, err = l.searchSpotifyFiltered(ctx, st, lavalink.SearchTypeYoutube, false, true)
	if err == nil {
		return track, nil
	}
	return nil, err
}

func (l *Lava) link(ctx context.Context, link string) ([]lavalink.AudioTrack, error) {
	rc := l.l.BestRestClient()
	if rc == nil {
		return nil, ErrNoRestClient
	}

	resp, err := rc.LoadItem(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("failed to load item: %w", err)
	}
	if resp.Exception != nil {
		return nil, fmt.Errorf("resp has friendly exception: %w", resp.Exception)
	}
	if len(resp.Tracks) == 0 {
		return nil, fmt.Errorf("no tracks returned for item")
	}

	parsed, err := l.parseTracks(resp.Tracks...)
	if err != nil {
		return nil, err
	}

	i := resp.PlaylistInfo.SelectedTrack
	if i != -1 {
		return []lavalink.AudioTrack{parsed[i]}, nil
	}
	return parsed, nil
}

func (l *Lava) parseTracks(tracks ...lavalink.LoadResultAudioTrack) ([]lavalink.AudioTrack, error) {
	var parsed []lavalink.AudioTrack
	for _, loadResultTrack := range tracks {
		track, err := l.l.DecodeTrack(loadResultTrack.Track)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, track)
	}
	return parsed, nil
}

func (l *Lava) Query(ctx context.Context, text string) ([]lavalink.AudioTrack, int, error) {
	_, err := url.ParseRequestURI(text)
	if err != nil {
		// First search youtube music
		t, err := l.search(ctx, lavalink.SearchTypeYoutubeMusic, text)
		if err == nil {
			return []lavalink.AudioTrack{t[0]}, 0, nil
		}
		// Second search youtube
		t, err = l.search(ctx, lavalink.SearchTypeYoutube, text)
		if err == nil {
			return []lavalink.AudioTrack{t[0]}, 0, nil
		}
		// Third search soundcloud
		t, err = l.search(ctx, lavalink.SearchTypeSoundCloud, text)
		if err == nil {
			return []lavalink.AudioTrack{t[0]}, 0, nil
		}
		return nil, 0, err
	} else {
		// If we have a URL we first check if it's a spotify url
		if !strings.Contains(text, "spotify.com") {
			// If we just have a normal link
			tracks, err := l.link(ctx, text)
			if err != nil {
				return nil, 0, err
			}
			return tracks, 0, nil
		}

		// If it is a spotify URL we attempt to download the tracks
		if l.s == nil {
			return nil, 0, errors.New("spotify is unsupported")
		}
		queries, err := l.s.Download(ctx, text)
		if err != nil {
			return nil, 0, err
		}

		// Only allow processing up to 3000 tracks
		if len(queries) > 3000 {
			return nil, 0, fmt.Errorf("spotify tracks length is larger than max limit of %d: %d", 3000, len(queries))
		}

		// Now we search for the tracks on youtube using spotify metadata
		var wg sync.WaitGroup
		ordered := make([]orderedTrack, 0)
		for i, q := range queries {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			default:
			}

			wg.Add(1)
			go func(o int, st spotifyTrack) {
				defer wg.Done()
				err := l.rl.Wait(ctx)
				if err != nil {
					log.Error().Err(err).Interface("track", st).Msg("failed to rate limit for spotify track")
					return
				}

				t, err := l.searchSpotify(ctx, st)
				if err != nil {
					log.Error().Err(err).Interface("track", st).Msg("failed to search for spotify track")
					return
				}

				ordered = append(ordered, orderedTrack{
					order: o,
					t:     t,
				})
			}(i, q)
		}
		wg.Wait()

		// Exit if no tracks
		if len(ordered) == 0 {
			return nil, 0, nil
		}

		// Sort and convert to tracks
		sort.SliceStable(ordered, func(i, j int) bool {
			return ordered[i].order < ordered[j].order
		})
		tracks := make([]lavalink.AudioTrack, len(ordered))
		for i := range ordered {
			tracks[i] = ordered[i].t
		}
		return tracks, len(queries) - len(ordered), nil
	}
}

func (l *Lava) EnsurePlayerExists(guildID discord.GuildID) {
	l.l.Player(snowflake.Snowflake(guildID.String()))
}

func (l *Lava) Play(ctx context.Context, guildID discord.GuildID, t lavalink.AudioTrack) (CloseEvent, error) {
	n := l.l.BestNode()
	if n == nil {
		return CloseEvent{Type: TrackEnd}, fmt.Errorf("node doesn't exist")
	}

	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return CloseEvent{Type: TrackEnd}, ErrNoPlayer
	}

	done := make(chan CloseEvent)
	listener := closeListener{quit: done}
	p.AddListener(listener)

	t.SetPosition(0)
	err := p.Play(t)
	if err != nil {
		return CloseEvent{Type: TrackEnd}, err
	}

	ce := CloseEvent{Type: TrackEnd, Error: "context done", Reason: "Context done"}
	select {
	case <-ctx.Done():
	case e := <-done:
		ce = e
	}

	p.RemoveListener(listener)
	close(done)

	return ce, p.Stop()
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

	return p.Seek(ConvertDuration(t))
}

func (l *Lava) Position(guildID discord.GuildID) (time.Duration, error) {
	p := l.l.Player(snowflake.Snowflake(guildID.String()))
	if p == nil {
		return 0, ErrNoPlayer
	}

	return ParseDuration(p.Position()), nil
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
