package ytdlp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type Track struct {
	sync.Mutex

	ID         string        `json:"id"`
	VideoTitle string        `json:"title"`
	Uploader   string        `json:"uploader"`
	Duration   time.Duration `json:"duration"`
	URL        string        `json:"url"`
	Title      string        `json:"track"`
	Artist     string        `json:"artist"`
	Album      string        `json:"album"`

	dlOnce    sync.Once
	abortOnce sync.Once
	abort     chan struct{}
	oggFile   chan []byte
}

func (t *Track) Abort() {
	t.Lock()
	defer t.Unlock()

	if t.abort == nil {
		return
	}

	go t.abortOnce.Do(func() {
		defer close(t.abort)

		t.abort <- struct{}{}
	})
}

func (t *Track) FileChan() <-chan []byte {
	return t.oggFile
}

func (t *Track) Download(c *Client) {
	t.Lock()
	defer t.Unlock()

	go t.dlOnce.Do(func() {
		t.abort = make(chan struct{})
		t.oggFile = make(chan []byte)
		defer close(t.oggFile)

		tLog := log.With().Str("track", t.Pretty()).Logger()

		// Download track
		tLog.Trace().Msg("starting to download")
		audio, err := c.DownloadFile(context.Background(), t.URL)
		if err != nil {
			tLog.Error().Err(err).Msg("failed to download file")
			select {
			case t.oggFile <- nil:
				tLog.Trace().Msg("sent nil track")
			case <-t.abort:
				tLog.Trace().Msg("aborted while waiting to send nil track")
			}
			return
		}
		tLog.Trace().Msg("successfully downloaded")

		// Send track to
		select {
		case t.oggFile <- audio:
			tLog.Trace().Msg("sent downloaded track")
		case <-t.abort:
			tLog.Trace().Msg("aborted while waiting to send track")
		}
	})
}

func (t *Track) Pretty() string {
	if t.Title != "" {
		return fmt.Sprintf("`%s` - `%s`", t.Artist, t.Title)
	}
	return fmt.Sprintf("`%s` - `%s`", t.Uploader, t.VideoTitle)
}
