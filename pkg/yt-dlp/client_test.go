package ytdlp

import (
	"context"
	"os"
	"testing"
)

const (
	youtube            = "https://youtu.be/dceGIpBtQZo"
	soundcloud         = "https://soundcloud.com/fractalfantasy/sinjin-hawke-blank-spaces-1"
	youtubePlaylist    = "https://www.youtube.com/playlist?list=PLwVziAzt2oDLTvZtj6O0mnagwpdZfhROW"
	soundcloudPlaylist = "https://soundcloud.com/zora-jones/sets/vicious-circles-sinjin-hawke-zora-jones"
)

var c = NewClient(os.Getenv("SPOTIFY_ID"), os.Getenv("SPOTIFY_SECRET"))
var ctx = context.TODO()

func TestLink(t *testing.T) {
	track, err := c.searchLink(ctx, youtube)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Youtube:", track)
	}

	track, err = c.searchLink(ctx, soundcloud)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Soundcloud:", track)
	}

	track, err = c.searchLink(ctx, youtubePlaylist)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Youtube Playlist:", track)
	}

	track, err = c.searchLink(ctx, soundcloudPlaylist)
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Soundcloud Playlist:", track)
	}
}

func TestSearch(t *testing.T) {
	tracks, err := c.searchQuery(ctx, "and you were one")
	if err != nil {
		t.Error(err)
	}
	t.Log("Search:", tracks)
}

func TestDownload(t *testing.T) {
	tracks, err := c.DownloadMetadata(ctx, "and you were one")
	if err != nil {
		t.Error(err)
	}
	t.Log("Download (search):", tracks)

	tracks, err = c.DownloadMetadata(ctx, "https://www.youtube.com/watch?v=LYzM3oWC8p8")
	if err != nil {
		t.Error(err)
	}
	t.Log("Download (link):", tracks)
}

func TestSpotifySearch(t *testing.T) {
	track, err := c.DownloadMetadata(ctx, "https://open.spotify.com/track/3Pb9QabepyR9e9D8NqorPH?si=4f75dad081f4430b")
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Track (spotify):", track)
	}

	track, err = c.DownloadMetadata(ctx, "https://open.spotify.com/album/0QMxX4ZCFZK3ku24sviec4?si=gYf5pWPZSm27FbzCJNzr6g")
	if err != nil {
		t.Error(err)
	} else {
		t.Log("Album (spotify):", track)
	}

}

func TestInvalidTracks(t *testing.T) {
	_, err := c.DownloadMetadata(ctx, "https://www.youtube.com/playlist?list=PLkLKCs4iHkejj-QVr2q_WLjOUueG3x5Es")
	if err == nil {
		t.Error(err)
	}
}
