package ogg

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

func TestOpus(t *testing.T) {
	// Read in the data
	data, err := os.ReadFile("organ.opus") // File from https://www.kozco.com/tech/soundtests.html
	if err != nil {
		t.Error(err)
	}

	// Create the reader and writer and decoder
	src := bytes.NewReader(data)
	var b bytes.Buffer
	dst := bufio.NewWriter(&b)
	d := NewDecoder()

	// Context for controlling the decoder
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Test the seeking to 0
		time.Sleep(2 * time.Second)
		err = d.Seek(0)
		if err != nil {
			t.Error(err)
		}

		// Test the seeking to arbitrary point
		time.Sleep(1 * time.Second)
		err = d.Seek(8 * time.Second)
		if err != nil {
			t.Error(err)
		}

		// Test seeking past end of file
		time.Sleep(1 * time.Second)
		err = d.Seek(5 * time.Minute)
		if err == nil {
			t.Error(err)
		}

		// Tell the decoder to stop
		cancel()
	}()

	// Run the decoding
	test = true
	err = d.Decode(ctx, dst, src)
	if err != nil {
		t.Error(err)
	}
}
