package ytdlp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type search struct {
	T    *Track
	Diff time.Duration
}

func (c *Client) searchLink(ctx context.Context, link string) ([]*Track, error) {
	buf, err := c.ytdlpMetadata(ctx, link, false)
	if err != nil {
		return nil, err
	}

	var tracks []*Track
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
		return []*Track{t}, nil
	}
}

func (c *Client) searchQuery(ctx context.Context, text string) (*Track, error) {
	buf, err := c.ytdlpMetadata(ctx, "ytsearch1:"+text, false)
	if err != nil {
		return nil, err
	}

	t, err := unmarshalPlaylist(buf)
	if err != nil {
		return nil, err
	}

	return t[0], nil
}

func (c *Client) searchSpotify(ctx context.Context, st spotifyTrack) (*Track, error) {
	// First we perform the search
	buf, err := c.ytdlpMetadata(ctx, fmt.Sprintf("ytsearch5: %s %s", st.Artist, st.Title), true)
	if err != nil {
		return nil, err
	}
	tracks, err := unmarshalPlaylist(buf)
	if err != nil {
		return nil, err
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
