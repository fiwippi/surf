package voice

import (
	"fmt"
	"testing"
)

func TestMove(t *testing.T) {
	q := newQueue()
	q.l.PushBack("fox")
	q.l.PushBack("yak")
	q.l.PushBack("emu")
	q.l.PushBack("jay")
	q.l.PushBack("koi")

	// Move to front
	_, err := q.Move(4, 0)
	if err != nil {
		t.Error(err)
	}
	e := q.l.Front()
	if e == nil {
		t.Error("front of queue is nil")
	}
	if e.Value.(string) != "koi" {
		t.Error("front of queue is not 'koi'")
	}

	// Move to back
	_, err = q.Move(0, 4)
	if err != nil {
		t.Error(err)
	}
	e = q.l.Back()
	if e == nil {
		t.Error("back of queue is nil")
	}
	if e.Value.(string) != "koi" {
		t.Error("back of queue is not 'koi'")
	}

	// Move to Middle
	_, err = q.Move(4, 1)
	if err != nil {
		t.Error(err)
	}
	e, err = q.element(1)
	if err != nil || e == nil {
		t.Error("error or back of queue is nil:", err)
	}
	if e.Value.(string) != "koi" {
		t.Error("second element is not 'koi'")
	}
}

func TestShuffle(t *testing.T) {
	q := newQueue()
	q.l.PushBack("fox")
	q.l.PushBack("yak")
	q.l.PushBack("emu")
	q.l.PushBack("jay")
	q.l.PushBack("koi")

	// Print before
	fmt.Println("Before:")
	count := 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(string))
		count++
	}

	// Do the shuffling
	q.Shuffle()

	// Print after
	fmt.Println("After:")
	count = 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(string))
		count++
	}
}
