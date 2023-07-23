package ytdlp

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/zmb3/spotify/v2"
)

var (
	sc *spotifyClient

	trackURI    spotify.ID = "5aH0gOvX64uaomC5TE2YJz"
	albumURI    spotify.ID = "2Vx9FC6Um8i6kEtY7HNswB"
	playlistURI spotify.ID = "37i9dQZF1DX8tZsk68tuDw"
)

func init() {
	sc = newSpotifyClient(os.Getenv("SPOTIFY_ID"), os.Getenv("SPOTIFY_SECRET"))
	if sc == nil {
		log.Fatal().Err(fmt.Errorf("could not create spotify client")).Send()
	}
}

func TestSpotifyTrack(t *testing.T) {
	st, err := sc.track(ctx, trackURI)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Track:", st)
}

func TestSpotifyAlbum(t *testing.T) {
	st, err := sc.album(ctx, albumURI)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Album:", st)
}

func TestSpotifyPlaylist(t *testing.T) {
	st, err := sc.playlist(ctx, playlistURI)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Playlist:", st)
}

func TestSpotifyDownload(t *testing.T) {
	st, err := sc.Download(ctx, "https://open.spotify.com/track/3Pb9QabepyR9e9D8NqorPH?si=4f75dad081f4430b")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Track (download len):", len(st))

	st, err = sc.Download(ctx, "https://open.spotify.com/album/0QMxX4ZCFZK3ku24sviec4?si=gYf5pWPZSm27FbzCJNzr6g")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Album (download len):", len(st))

	st, err = sc.Download(ctx, "https://open.spotify.com/playlist/37i9dQZF1DX4dyzvuaRJ0n?si=3c8edaa8116a4f14")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Spotify Playlist (download len):", len(st))
}
