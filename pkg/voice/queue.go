package voice

import (
	"container/list"
	"errors"
	"fmt"

	"surf/pkg/ytdlp"
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

func (q *queue) Pop() (ytdlp.Track, error) {
	e := q.l.Front()
	if e == nil {
		return ytdlp.Track{}, ErrEmptyQueue
	}

	q.l.Remove(e)
	return e.Value.(ytdlp.Track), nil
}

func (q *queue) Push(t ytdlp.Track) {
	q.l.PushBack(t)
}

func (q *queue) Remove(i int) error {
	if i < 0 || i >= q.Len() {
		return errors.New("element does not exist")
	}

	count := 0
	for e := q.l.Front(); e != nil; e = e.Next() {
		if i == count {
			q.l.Remove(e)
			return nil
		}
		count++
	}

	panic(fmt.Errorf("queue element '%d' should exist", i))
}

func (q *queue) Tracks() []ytdlp.Track {
	tracks := make([]ytdlp.Track, 0)
	for e := q.l.Front(); e != nil; e = e.Next() {
		t := e.Value.(ytdlp.Track)
		tracks = append(tracks, t)
	}
	return tracks
}

func (q *queue) Move(i, j int) error {
	if i < 0 || i >= q.Len() || j < 0 || j >= q.Len() {
		return errors.New("element does not exist")
	} else if i == j {
		return nil
	}

	e, err := q.element(i)
	if err != nil {
		return err
	}

	if j == 0 {
		q.l.MoveToFront(e)
	} else if j == q.Len() - 1 {
		q.l.MoveToBack(e)
	} else {
		f, err := q.element(j + 1)
		if err != nil {
			return err
		}
		q.l.MoveAfter(e, f)
	}

	return nil
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