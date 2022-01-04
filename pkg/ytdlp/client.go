package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"surf/internal/log"
)

const MaxRequestsPerSec = 3

type Client struct {
	rl      *rate.Limiter
	spotify *spotifyClient
}

type search struct {
	T    Track
	Diff time.Duration
}

func NewClient() *Client {
	c := &Client{
		rl: rate.NewLimiter(rate.Every(time.Second/time.Duration(MaxRequestsPerSec)), 1),
	}

	s, err := newSpotifyClient()
	if err == nil {
		c.spotify = s
	}

	return c
}

func (c *Client) DownloadMetadata(ctx context.Context, text string) ([]Track, error) {
	_, err := url.ParseRequestURI(text)
	if err != nil {
		// If we have a search term
		t, err := c.search(ctx, text)
		if err != nil {
			return nil, err
		}
		return []Track{t}, nil
	} else {
		// If we have a URL we first check if it's a spotify url
		if !strings.Contains(text, "spotify.com") {
			tracks, err := c.link(ctx, text)
			if err != nil {
				return nil, err
			}
			return tracks, nil
		}

		// If it is a spotify URL we attempt to download the tracks
		if c.spotify == nil {
			return nil, errors.New("spotify is unsupported")
		}
		queries, err := c.spotify.Download(ctx, text)
		if err != nil {
			return nil, err
		}

		// Now we search for the tracks on youtube using spotify metadata
		tracks := make([]Track, 0)
		for _, q := range queries {
			t, err := c.searchComplex(ctx, q)
			if err != nil {
				log.Error().Err(err).Interface("track", t).Msg("failed to search for spotify track with yt-dlp")
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

func (c *Client) DownloadFile(ctx context.Context, url string) ([]byte, error) {
	err := c.rl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	// Download the audio
	dl := exec.CommandContext(ctx,
		"yt-dlp", "-q", "-v", "-f", "ba[vcodec=none]",
		"--compat-options", "no-youtube-unavailable-videos",
		"-o", "-", url,
	)
	audio, err := dl.Output()
	if err != nil {
		return nil, err
	}

	// Encode the audio into opus
	encode := exec.CommandContext(ctx,
		"ffmpeg", "-i", "-",
		"-hide_banner", "-loglevel", "error", "-vn",
		"-c:a", "libopus", "-b:a", "96k", "-vbr", "off", "-application", "audio",
		"-f", "opus", "-",
	)
	encode.Stdin = bytes.NewReader(audio)
	encAudio, err := encode.Output()
	if err != nil {
		return nil, err
	}
	return encAudio, nil
}

func (c *Client) searchComplex(ctx context.Context, st spotifyTrack) (Track, error) {
	// First we perform the search
	buf, err := c.ytdlpMetadata(ctx, fmt.Sprintf("ytsearch5: %s %s", st.Artist, st.Title), true)
	if err != nil {
		return Track{}, err
	}
	tracks, err := unmarshalPlaylist(buf)
	if err != nil {
		return Track{}, err
	}

	// Now filter the searches
	filtered := make([]search, 0)
	for _, track := range tracks {
		diff := st.Duration - track.Duration
		if diff < 0 {
			diff *= -1
		}

		containsArtist := strings.Contains(track.Artist, st.Artist)
		containsTitle := strings.Contains(track.Title, st.Title)
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

func (c *Client) search(ctx context.Context, text string) (Track, error) {
	buf, err := c.ytdlpMetadata(ctx, "ytsearch1:"+text, false)
	if err != nil {
		return Track{}, err
	}

	t, err := unmarshalPlaylist(buf)
	if err != nil {
		return Track{}, err
	}
	return t[0], nil
}

func (c *Client) link(ctx context.Context, link string) ([]Track, error) {
	buf, err := c.ytdlpMetadata(ctx, link, false)
	if err != nil {
		return nil, err
	}

	var tracks []Track
	if isPlaylist(buf) {
		tracks, err = unmarshalPlaylist(buf)
		if err != nil {
			return nil, err
		}
		return tracks, nil
	} else {
		t, err := unmarshalTrack(buf)
		if err != nil {
			return nil, err
		}
		return []Track{t}, nil
	}
}

func (c *Client) ytdlpMetadata(ctx context.Context, query string, unflatten bool) ([]byte, error) {
	err := c.rl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-J", "-i", "-f", "ba[vcodec=none]",
		"--no-playlist", "--no-warnings",
		"--compat-options", "no-youtube-unavailable-videos",
		"--flat-playlist",
	}
	if strings.Contains(query, "soundcloud.com") || unflatten {
		// For soundcloud we need to remove --flat-playlist otherwise
		// we don't get the full playlist data
		args[len(args)-1] = query
	} else {
		args = append(args, query)
	}

	dl := exec.CommandContext(ctx, "yt-dlp", args...)
	buf, err := dl.Output()
	if err != nil {
		return nil, err
	}
	return buf, nil
}
