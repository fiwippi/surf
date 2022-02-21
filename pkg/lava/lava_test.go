package lava

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"

	"surf/internal/log"
)

const (
	youtube                 = "https://youtu.be/dceGIpBtQZo"
	soundcloud              = "https://soundcloud.com/fractalfantasy/sinjin-hawke-blank-spaces-1"
	youtubePlaylist         = "https://www.youtube.com/playlist?list=PLwVziAzt2oDLTvZtj6O0mnagwpdZfhROW"
	soundcloudPlaylist      = "https://soundcloud.com/zora-jones/sets/vicious-circles-sinjin-hawke-zora-jones"
	youtubePlaylistSpecific = "https://www.youtube.com/watch?v=JDvouXlfmzw&list=OLAK5uy_laAbq3Jk4VgDo9_YBL9I-gnNUga5mHpQA&index=6"
)

var lava *Lava

func TestMain(m *testing.M) {
	conf := Config{
		Host:          "0.0.0.0",
		Port:          "2333",
		Pass:          "lava",
		SpotifyID:     os.Getenv("SPOTIFY_ID"),
		SpotifySecret: os.Getenv("SPOTIFY_SECRET"),
	}

	var err error
	lava, err = NewLava(conf)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create lavalink client")
	}
	os.Exit(m.Run())
}

func fmtTrack(tracks ...lavalink.AudioTrack) string {
	str := make([]string, 0)
	for _, t := range tracks {
		str = append(str, fmt.Sprintf("%s - %s (%s)", t.Info().Author, t.Info().Title, t.Info().Length))
	}

	return strings.Join(str, ", ")
}

func TestLinks(t *testing.T) {
	tracks, err := lava.link(context.Background(), youtube)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to play youtube link")
	}
	fmt.Println("Track - Youtube: ", fmtTrack(tracks...))

	tracks, err = lava.link(context.Background(), soundcloud)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to play soundcloud link")
	}
	fmt.Println("Track - Soundcloud: ", fmtTrack(tracks...))

	tracks, err = lava.link(context.Background(), youtubePlaylist)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to play youtube playlist link")
	}
	fmt.Println("Track - Youtube Playlist: ", fmtTrack(tracks...))

	tracks, err = lava.link(context.Background(), soundcloudPlaylist)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to play soundcloud playlist link")
	}
	fmt.Println("Track - Soundcloud Playlist: ", fmtTrack(tracks...))

	tracks, err = lava.link(context.Background(), youtubePlaylistSpecific)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to play youtube playlist link with specific track")
	}
	fmt.Println("Track - Youtube Playlist Specific: ", fmtTrack(tracks...))
}

func TestSearch(t *testing.T) {
	tracks, err := lava.search(context.Background(), lavalink.SearchTypeYoutube, "1999 she")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to search youtube track")
	}
	fmt.Println("Search - Youtube: ", fmtTrack(tracks[0]))

	tracks, err = lava.search(context.Background(), lavalink.SearchTypeYoutubeMusic, "nariaki obukuro gaia")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to search youtube music track")
	}
	fmt.Println("Search - Youtube Music: ", fmtTrack(tracks[0]))

	tracks, err = lava.search(context.Background(), lavalink.SearchTypeYoutubeMusic, "sinjin hawke blank spaces")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to search soundcloud track")
	}
	fmt.Println("Search - Soundcloud: ", fmtTrack(tracks[0]))
}

func TestDownload(t *testing.T) {
	tracks, _, err := lava.Query(ctx, "sinjin and you were one")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Download (search): %s\n", fmtTrack(tracks...))

	tracks, _, err = lava.Query(ctx, "https://www.youtube.com/watch?v=LYzM3oWC8p8")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Download (link): %s\n", fmtTrack(tracks...))
}

func TestSpotifySearch(t *testing.T) {
	tracks, _, err := lava.Query(ctx, "https://open.spotify.com/track/3Pb9QabepyR9e9D8NqorPH?si=4f75dad081f4430b")
	if err != nil {
		t.Error(err)
	} else {
		fmt.Printf("Track (Spotify): %s\n", fmtTrack(tracks[0]))
	}

	tracks, _, err = lava.Query(ctx, "https://open.spotify.com/album/0QMxX4ZCFZK3ku24sviec4?si=gYf5pWPZSm27FbzCJNzr6g")
	if err != nil {
		t.Error(err)
	} else {
		fmt.Printf("Album (Spotify): %s\n", fmtTrack(tracks...))
	}

}

func TestInvalidTracks(t *testing.T) {
	_, _, err := lava.Query(ctx, "https://www.youtube.com/playlist?list=PLkLKCs4iHkejj-QVr2q_WLjOUueG3x5Es")
	if err == nil {
		t.Error(err)
	}
}

func TestDownloadLargeSpotifyPlaylist(t *testing.T) {
	start := time.Now()
	_, _, err := lava.Query(ctx, "https://open.spotify.com/playlist/647no1muSFW2iJe2mrQAc0?si=903e24bd0f0645a8")
	if err != nil {
		t.Error(err)
	}
	duration := time.Since(start)

	fmt.Printf("Download (spotify - large playlist): %s\n", duration)
}
