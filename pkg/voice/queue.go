package voice

import (
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/DisgoOrg/disgolink/lavalink"
)

var ErrEmptyQueue = errors.New("queue is empty")

type queue struct {
	l *list.List
}

func newQueue() *queue {
	return &queue{l: list.New()}
}

func (q *queue) Init() { q.l.Init() }

func (q *queue) Len() int { return q.l.Len() }

func (q *queue) Pop() (lavalink.AudioTrack, error) {
	e := q.l.Front()
	if e == nil {
		return nil, ErrEmptyQueue
	}

	q.l.Remove(e)
	return e.Value.(lavalink.AudioTrack), nil
}

func (q *queue) Push(t lavalink.AudioTrack) {
	q.l.PushBack(t)
}

func (q *queue) Remove(i, j int) ([]lavalink.AudioTrack, error) {
	if i < 0 || i >= q.Len() {
		return nil, errors.New("element does not exist")
	}
	if j < i {
		return nil, errors.New("cannot remove in negative range")
	}

	removed := make([]lavalink.AudioTrack, 0)

	count := 0
	for e := q.l.Front(); e != nil; count++ {
		tmp := e
		e = e.Next()
		if count >= i && count <= j {
			q.l.Remove(tmp)
			removed = append(removed, tmp.Value.(lavalink.AudioTrack))
		}
	}
	if len(removed) > 0 {
		return removed, nil
	}

	panic(fmt.Errorf("queue element '%d' should exist", i))
}

func (q *queue) Tracks() []lavalink.AudioTrack {
	tracks := make([]lavalink.AudioTrack, 0)
	for e := q.l.Front(); e != nil; e = e.Next() {
		t := e.Value.(lavalink.AudioTrack)
		tracks = append(tracks, t)
	}
	return tracks
}

func (q *queue) Move(i, j int) (lavalink.AudioTrack, error) {
	if i < 0 || i >= q.Len() || j < 0 || j >= q.Len() {
		return nil, errors.New("element does not exist")
	}

	e, err := q.element(i)
	if err != nil {
		return nil, err
	}
	if i == j {
		return e.Value.(lavalink.AudioTrack), nil
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

	return e.Value.(lavalink.AudioTrack), nil
}

func (q *queue) Shuffle() {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(q.l.Len(), func(i, j int) {
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
