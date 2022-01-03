package ytdlp

import (
	"errors"
	"time"

	"github.com/diamondburned/arikawa/v3/utils/json"
)

func unmarshalTrack(b []byte) (Track, error) {
	temp := struct {
		Track
		Duration   float64 `json:"duration"`
		WebpageURL string  `json:"webpage_url"`
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return Track{}, err
	}
	if temp.Track == (Track{}) {
		return Track{}, errors.New("invalid track downloaded")
	}

	t := temp.Track
	t.URL = temp.WebpageURL
	t.Duration = time.Duration(temp.Duration) * time.Second
	return t, nil
}

func unmarshalPlaylist(b []byte) ([]Track, error) {
	p :=  struct {
		Entries []struct {
			Track
			Duration   float64 `json:"duration"`
			WebpageURL string `json:"webpage_url"`
		} `json:"entries"`
	}{}
	err := json.Unmarshal(b, &p)
	if err != nil {
		return nil, err
	} else if len(p.Entries) < 1 {
		return nil, errors.New("playlist has no entries")
	} else if p.Entries[0].Track == (Track{}) {
		return nil, errors.New("invalid track")
	}

	tracks := make([]Track, len(p.Entries))
	for i := range p.Entries {
		tracks[i] = p.Entries[i].Track
		tracks[i].Duration = time.Duration(p.Entries[i].Duration) * time.Second
		if p.Entries[i].WebpageURL != "" {
			tracks[i].URL = p.Entries[i].WebpageURL
		}
	}
	return tracks, nil
}

func isPlaylist(b []byte) bool {
	resp := struct {
		Type string `json:"_type"`
	}{}
	err := json.Unmarshal(b, &resp)
	if err != nil {
		return false
	}
	return resp.Type == "playlist"
}
