package ytdlp

import (
	"fmt"
	"time"
)

type Track struct {
	ID         string        `json:"id"`
	VideoTitle string        `json:"title"`
	Uploader   string        `json:"uploader"`
	Duration   time.Duration `json:"duration"`
	URL        string        `json:"url"`
	Title      string        `json:"track"`
	Artist     string        `json:"artist"`
	Album      string        `json:"album"`
}

func (t *Track) Pretty() string {
	if t.Title != "" {
		return fmt.Sprintf("`%s` - `%s`", t.Artist, t.Title)
	}
	return fmt.Sprintf("`%s` - `%s`", t.Uploader, t.VideoTitle)
}