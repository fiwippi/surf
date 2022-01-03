package ytdlp

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"
)

type spotifyClient struct {
	rl     *rate.Limiter
	client *spotify.Client
}

type spotifyTrack struct {
	Artist   string
	Title    string
	Duration time.Duration
}

func newSpotifyClient() (*spotifyClient, error) {
	id := os.Getenv("SPOTIFY_ID")
	if id == "" {
		return nil, errors.New("no $SPOTIFY_ID given")
	}
	secret := os.Getenv("SPOTIFY_SECRET")
	if secret == "" {
		return nil, errors.New("no $SPOTIFY_SECRET given")
	}

	ctx := context.Background()
	config := &clientcredentials.Config{
		ClientID:     id,
		ClientSecret: secret,
		TokenURL:     spotifyauth.TokenURL,
	}
	token, err := config.Token(ctx)
	if err != nil {
		return nil, err
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	return &spotifyClient{
		client: client,
		rl:     rate.NewLimiter(rate.Every(time.Second/time.Duration(MaxRequestsPerSec)), 1),
	}, nil
}

func (s *spotifyClient) Download(ctx context.Context, link string) ([]spotifyTrack, error) {
	u, err := url.ParseRequestURI(link)
	if err != nil {
		return nil, err
	}

	err = s.rl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	if strings.Contains(u.Path, "track") {
		t, err := s.track(ctx, spotify.ID(strings.TrimPrefix(u.Path, "/track/")))
		if err != nil {
			return nil, err
		}
		return []spotifyTrack{t}, nil
	} else if strings.Contains(u.Path, "album") {
		return s.album(ctx, spotify.ID(strings.TrimPrefix(u.Path, "/album/")))
	} else if strings.Contains(u.Path, "playlist") {
		return s.playlist(ctx, spotify.ID(strings.TrimPrefix(u.Path, "/playlist/")))
	}

	return nil, errors.New("invalid spotify link type")
}

func (s *spotifyClient) spotifyTrack(t interface{}) spotifyTrack {
	var st spotifyTrack

	full, ok := t.(*spotify.FullTrack)
	if ok {
		st.Artist = full.Artists[0].Name
		st.Title = full.Name
		st.Duration = full.TimeDuration()
	}
	simple, ok := t.(*spotify.SimpleTrack)
	if ok {
		st.Artist = simple.Artists[0].Name
		st.Title = simple.Name
		st.Duration = simple.TimeDuration()
	}

	return st
}

func (s *spotifyClient) track(ctx context.Context, uri spotify.ID) (spotifyTrack, error) {
	t, err := s.client.GetTrack(ctx, uri)
	if err != nil {
		return spotifyTrack{}, err
	}
	return s.spotifyTrack(t), nil
}

func (s *spotifyClient) album(ctx context.Context, uri spotify.ID) ([]spotifyTrack, error) {
	tracks, err := s.client.GetAlbumTracks(ctx, uri)
	if err != nil {
		return nil, err
	}

	data := make([]spotifyTrack, len(tracks.Tracks))
	for i, t := range tracks.Tracks {
		data[i] = s.spotifyTrack(&t)
	}
	return data, nil
}

func (s *spotifyClient) playlist(ctx context.Context, uri spotify.ID) ([]spotifyTrack, error) {
	tracks, err := s.client.GetPlaylistTracks(ctx, uri)
	if err != nil {
		return nil, err
	}

	totalTracks := make([]spotifyTrack, 0)
	for page := 1; ; page++ {
		// Make the data
		data := make([]spotifyTrack, len(tracks.Tracks))
		for i, t := range tracks.Tracks {
			data[i] = s.spotifyTrack(&t.Track)
		}
		totalTracks = append(totalTracks, data...)

		// Get the next page if applicable
		err = s.client.NextPage(ctx, tracks)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return totalTracks, nil
}