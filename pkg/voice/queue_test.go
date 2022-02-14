package voice

import (
	"fmt"
	"testing"

	"github.com/DisgoOrg/disgolink/lavalink"
)

func testAudioTrack(title string) lavalink.AudioTrack {
	return lavalink.NewAudioTrack(lavalink.AudioTrackInfo{Title: title})
}

func TestMove(t *testing.T) {
	q := newQueue()
	q.l.PushBack(testAudioTrack("fox"))
	q.l.PushBack(testAudioTrack("yak"))
	q.l.PushBack(testAudioTrack("emu"))
	q.l.PushBack(testAudioTrack("jay"))
	q.l.PushBack(testAudioTrack("koi"))

	// Move to front
	_, err := q.Move(4, 0)
	if err != nil {
		t.Error(err)
	}
	e := q.l.Front()
	if e == nil {
		t.Error("front of queue is nil")
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "koi" {
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
	if e.Value.(lavalink.AudioTrack).Info().Title != "koi" {
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
	if e.Value.(lavalink.AudioTrack).Info().Title != "koi" {
		t.Error("second element is not 'koi'")
	}
}

func TestShuffle(t *testing.T) {
	q := newQueue()
	q.l.PushBack(testAudioTrack("fox"))
	q.l.PushBack(testAudioTrack("yak"))
	q.l.PushBack(testAudioTrack("emu"))
	q.l.PushBack(testAudioTrack("jay"))
	q.l.PushBack(testAudioTrack("koi"))

	// Print before
	fmt.Println("Before (shuffle):")
	count := 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(lavalink.AudioTrack).Info().Title)
		count++
	}

	// Do the shuffling
	q.Shuffle()

	// Print after
	fmt.Println("After (shuffle):")
	count = 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(lavalink.AudioTrack).Info().Title)
		count++
	}
}

func TestRemove(t *testing.T) {
	q := newQueue()
	q.l.PushBack(testAudioTrack("fox"))
	q.l.PushBack(testAudioTrack("yak"))
	q.l.PushBack(testAudioTrack("emu"))
	q.l.PushBack(testAudioTrack("jay"))
	q.l.PushBack(testAudioTrack("koi"))

	// Remove from front
	_, err := q.Remove(0, 0)
	if err != nil {
		t.Error(err)
	}
	e := q.l.Front()
	if e == nil {
		t.Error("front of queue is nil")
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "yak" {
		t.Error("front of queue is not 'yak'")
	}

	// Remove from back
	_, err = q.Remove(3, 3)
	if err != nil {
		t.Error(err)
	}
	e = q.l.Back()
	if e == nil {
		t.Error("back of queue is nil")
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "jay" {
		t.Error("back of queue is not 'jay':", e.Value.(lavalink.AudioTrack).Info().Title)
	}

	// Remove from middle
	r, err := q.Remove(1, 1)
	if err != nil {
		t.Error(err)
	}
	e, err = q.element(1)
	if err != nil || e == nil {
		t.Error("error or back of queue is nil:", err)
	}
	if r[0].Info().Title != "emu" {
		t.Error("removed track is not 'emu'")
	}

	// Add some tracks and try to remove in range

	q.l.PushBack(testAudioTrack("cat"))
	q.l.PushBack(testAudioTrack("dog"))
	q.l.PushBack(testAudioTrack("horse"))
	q.l.PushBack(testAudioTrack("rabbit"))
	fmt.Println("Before (remove):")
	count := 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(lavalink.AudioTrack).Info().Title)
		count++
	}
	r, err = q.Remove(2, 4)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("After (remove 1):")
	count = 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(lavalink.AudioTrack).Info().Title)
		count++
	}

	e, err = q.element(0)
	if err != nil || e == nil {
		t.Error("error or front of queue is nil:", err)
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "yak" {
		t.Error("front of queue is not 'yak'")
	}

	e, err = q.element(1)
	if err != nil || e == nil {
		t.Error("error or middle of queue is nil:", err)
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "jay" {
		t.Error("middle of queue is not 'jay'")
	}

	e, err = q.element(2)
	if err != nil || e == nil {
		t.Error("error or back of queue is nil:", err)
	}
	if e.Value.(lavalink.AudioTrack).Info().Title != "rabbit" {
		t.Error("back of queue is not 'rabbit'")
	}

	if len(r) != 3 {
		t.Error("did not remove 3 elements:", len(r))
	}
	if r[0].Info().Title != "cat" {
		t.Error("did not remove 'cat' from queue")
	}
	if r[1].Info().Title != "dog" {
		t.Error("did not remove 'dog' from queue")
	}
	if r[2].Info().Title != "horse" {
		t.Error("did not remove 'horse' from queue")
	}

	// Remove the rest of the tracks
	r, err = q.Remove(0, 2)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("After (remove 2):")
	count = 1
	for e := q.l.Front(); e != nil; e = e.Next() {
		fmt.Printf("%d. %s\n", count, e.Value.(lavalink.AudioTrack).Info().Title)
		count++
	}
	if q.Len() != 0 {
		t.Error("queue should be empty but has length:", q.Len())
	}
	if len(r) != 3 {
		t.Error("did not remove 3 elements:", len(r))
	}
	if r[0].Info().Title != "yak" {
		t.Error("did not remove 'yak' from queue")
	}
	if r[1].Info().Title != "jay" {
		t.Error("did not remove 'jay' from queue")
	}
	if r[2].Info().Title != "rabbit" {
		t.Error("did not remove 'rabbit' from queue")
	}
}
