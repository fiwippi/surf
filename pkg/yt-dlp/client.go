package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const MaxRequestsPerSec = 3

type Client struct {
	rl      *rate.Limiter
	spotify *spotifyClient
}

func NewClient(spotifyID, spotifySecret string) *Client {
	c := &Client{
		rl:      rate.NewLimiter(rate.Every(time.Second/time.Duration(MaxRequestsPerSec)), 1),
		spotify: newSpotifyClient(spotifyID, spotifySecret),
	}

	return c
}

func (c *Client) DownloadMetadata(ctx context.Context, text string) ([]*Track, error) {
	url, err := url.ParseRequestURI(text)

	// If we don't have a proper URL we treat the query as a search
	if err != nil {
		t, err := c.searchQuery(ctx, text)
		if err != nil {
			return nil, err
		}
		return []*Track{t}, nil
	}

	log.Debug().Str("query", text).Str("host", url.Host).Msg("valid url received")

	// Ensure the URL is from a valid domain
	isSpotify := strings.HasSuffix(url.Host, "spotify.com")
	isSoundcloud := url.Host == "soundcloud.com"
	isBandcamp := strings.HasSuffix(url.Host, ".bandcamp.com")
	isYouTube := url.Host == "youtu.be" || url.Host == "www.youtube.com"
	if !(isSoundcloud || isSpotify || isBandcamp || isYouTube) {
		return nil, errors.New("url is from an unsupported domain")
	}

	// If we don't have a spotify URL we treat it as just a link
	if !isSpotify {
		tracks, err := c.searchLink(ctx, text)
		if err != nil {
			return nil, err
		}
		return tracks, nil
	}

	// If it is a spotify URL we attempt to download the tracks' metadata
	// and then search on yt using said metadata
	if c.spotify == nil {
		return nil, errors.New("spotify is unsupported")
	}

	queries, err := c.spotify.Download(ctx, text)
	if err != nil {
		return nil, err
	}

	tracks := make([]*Track, 0)
	for _, q := range queries {
		t, err := c.searchSpotify(ctx, q)
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

func (c *Client) DownloadFile(ctx context.Context, url string) ([]byte, error) {
	err := c.rl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	// Download the audio
	dl := exec.CommandContext(ctx,
		"yt-dlp", "-q", "-v", "-f", "ba[vcodec=none]",
		"--proxy", os.Getenv("PROXY"), // If undefined this simply performs a direct connection
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

func (c *Client) ytdlpMetadata(ctx context.Context, query string, unflatten bool, extraArgs ...string) ([]byte, error) {
	err := c.rl.Wait(ctx)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-J", "-i", "-f", "ba[vcodec=none]",
		"--proxy", os.Getenv("PROXY"), // If undefined this simply performs a direct connection
		"--no-playlist", "--no-warnings",
		"--compat-options", "no-youtube-unavailable-videos",
		"--flat-playlist",
	}
	if strings.Contains(query, "soundcloud.com") || unflatten {
		// For soundcloud we need to remove --flat-playlist otherwise
		// we don't get the full playlist data
		args = args[:len(args)-1]
	}
	for _, eArg := range extraArgs {
		args = append(args, eArg)
	}
	args = append(args, query)

	dl := exec.CommandContext(ctx, "yt-dlp", args...)
	buf, err := dl.Output()
	if err != nil {
		return nil, err
	}
	return buf, nil
}
