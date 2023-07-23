package voice

import (
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"time"

	ytdlp "surf/pkg/yt-dlp"
)

var ErrEmptyQueue = errors.New("queue is empty")

type queue struct {
	l  *list.List
	yt *ytdlp.Client
}

func newQueue(yt *ytdlp.Client) *queue {
	return &queue{l: list.New(), yt: yt}
}

func (q *queue) Init() {
	tracks := q.Tracks()
	for _, t := range tracks {
		t.Abort()
	}
	q.l.Init()
}

func (q *queue) Len() int { return q.l.Len() }

func (q *queue) Pop() (*ytdlp.Track, error) {
	defer q.bufferAppropriateDls()

	e := q.l.Front()
	if e == nil {
		return nil, ErrEmptyQueue
	}
	q.l.Remove(e)

	t := e.Value.(*ytdlp.Track)
	return t, nil
}

func (q *queue) PushFront(tracks ...*ytdlp.Track) {
	defer q.bufferAppropriateDls()

	newTracks := list.New()
	for _, t := range tracks {
		newTracks.PushBack(t)
	}
	q.l.PushFrontList(newTracks)
}

func (q *queue) PushBack(tracks ...*ytdlp.Track) {
	defer q.bufferAppropriateDls()

	for _, t := range tracks {
		q.l.PushBack(t)
	}
}

func (q *queue) Remove(i, j int) ([]*ytdlp.Track, error) {
	if i < 0 || i >= q.Len() {
		return nil, errors.New("element does not exist")
	}
	if j < i {
		return nil, errors.New("cannot remove in negative range")
	}

	removed := make([]*ytdlp.Track, 0)

	count := 0
	for e := q.l.Front(); e != nil; count++ {
		tmp := e
		e = e.Next()
		if count >= i && count <= j {
			q.l.Remove(tmp)
			removedTrack := tmp.Value.(*ytdlp.Track)
			removedTrack.Abort()
			removed = append(removed, removedTrack)
		}
	}
	if len(removed) > 0 {
		return removed, nil
	}

	panic(fmt.Errorf("queue element '%d' should exist", i))
}

func (q *queue) Tracks() []*ytdlp.Track {
	tracks := make([]*ytdlp.Track, 0)
	for e := q.l.Front(); e != nil; e = e.Next() {
		t := e.Value.(*ytdlp.Track)
		tracks = append(tracks, t)
	}
	return tracks
}

func (q *queue) Move(i, j int) (*ytdlp.Track, error) {
	if i < 0 || i >= q.Len() || j < 0 || j >= q.Len() {
		return nil, errors.New("element does not exist")
	}

	e, err := q.element(i)
	if err != nil {
		return nil, err
	}
	if i == j {
		return e.Value.(*ytdlp.Track), nil
	}

	if j == 0 {
		q.l.MoveToFront(e)
	} else if j == q.Len()-1 {
		q.l.MoveToBack(e)
	} else {
		f, err := q.element(j - 1)
		if err != nil {
			return nil, err
		}
		q.l.MoveAfter(e, f)
	}

	return e.Value.(*ytdlp.Track), nil
}

func (q *queue) Shuffle() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(q.l.Len(), func(i, j int) {
		a, _ := q.element(i)
		b, _ := q.element(j)

		if a != nil && b != nil {
			a.Value, b.Value = b.Value, a.Value
		}
	})
}

func (q *queue) element(i int) (*list.Element, error) {
	if i < 0 || i >= q.Len() {
		return nil, errors.New("element does not exist")
	}

	count := 0
	for e := q.l.Front(); e != nil; e = e.Next() {
		if i == count {
			return e, nil
		}
		count++
	}

	panic(fmt.Errorf("queue element '%d' should exist", i))
}

func (q *queue) bufferAppropriateDls() {
	count := 0
	e := q.l.Front()

	for {
		if e == nil || count >= 3 {
			return
		}
		t := e.Value.(*ytdlp.Track)
		t.Download(q.yt)

		e = e.Next()
		count++
	}
}
