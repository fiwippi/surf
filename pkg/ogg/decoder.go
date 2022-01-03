// Code originally written by Steve McCoy under the MIT license and altered by
// Jonas747. The bulk of those was removed and the rest rewritten by diamondburned.
// The code has been rewritten again by me (fiwippi)

// Package ogg provides a decoder to unwrap opus frames into packets and to seek
// whilst the decoder is running
package ogg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"surf/internal/sync"
)

var test = false

type Decoder struct {
	// Controls decoding seeking so they don't happen concurrently
	c *sync.Controller
	// buffer holds the data we are reading from the src
	buffer []byte
	// src is the reader which contains the ogg data
	src io.ReadSeeker
	// Time is updated with the current timestamp we are on when decoding
	Time time.Duration
}

func NewDecoder() *Decoder {
	return &Decoder{
		c:      sync.NewController(),
		buffer: make([]byte, MaxPageSize),
	}
}

// Public

func (d *Decoder) Decode(ctx context.Context, dst io.Writer, src io.ReadSeeker) error {
	d.src = src
	err := d.decode(ctx, dst)
	if err == io.EOF {
		return nil
	}
	return err
}

func (d *Decoder) Seek(goal time.Duration) error {
	d.Pause()
	defer d.Resume()

	if d.src == nil {
		return nil
	}
	offset, err := d.seek(goal)
	if err == io.EOF {
		_, err = d.src.Seek(offset, io.SeekStart)
		return err
	}
	return err
}

func (d *Decoder) Pause() {
	time.Sleep(1 * time.Second)
	d.c.Pause()
}

func (d *Decoder) Resume() {
	time.Sleep(1 * time.Second)
	d.c.Resume()
}

// Private

func (d *Decoder) decode(ctx context.Context, dst io.Writer) error {
	var err error
	var granule uint64
	var nsegs = new(int)
	var packetBuf, segTblBuf []byte

	for {
		d.c.WaitIfPaused()

		select {
		case <-ctx.Done():
			return nil
		default:
			if test {
				time.Sleep(500 * time.Millisecond)
			}

			// Check if we should update the offset
			d.Time = time.Duration(float64(granule) / SampleRate) * time.Second


			// Read in the data from the src
			_, granule, err = d.readPage(&packetBuf, &segTblBuf, nsegs)
			if err != nil {
				return err
			}

			// Write the data to the destination
			err = d.writePage(&packetBuf, &segTblBuf, nsegs, dst)
			if err != nil {
				return err
			}
		}
	}
}

func (d *Decoder) seek(goal time.Duration) (int64, error) {
	var total, n int
	var granule uint64
	var byteOffset int64
	var minDifference = goal

	var err error
	var nsegs = new(int)
	var packetBuf, segTblBuf []byte

	// First we seek back to the start of the file
	o, err := d.src.Seek(0, io.SeekStart)
	if err != nil {
		return o, err
	}

	for {
		// Check if we should update the offset
		current := time.Duration(float64(granule) / SampleRate) * time.Second
		difference := goal - current
		if difference < 0 {
			difference *= -1
		}
		if difference < minDifference {
			minDifference = difference
			byteOffset = int64(total)
		}

		// Read the next ogg page
		n, granule, err = d.readPage(&packetBuf, &segTblBuf, nsegs)
		if err != nil {
			if goal > current {
				return 0, errors.New("the seek duration specified is invalid")
			}
			return byteOffset, err
		}
		total += n
	}
}

func (d *Decoder) readPage(packetBuf, segTblBuf *[]byte, nsegs *int) (int, uint64, error) {
	var n int
	var header pageHeader
	headerBuf := d.buffer[:HeaderSize]

	// Read in the data into the header buffer
	b, err := io.ReadFull(d.src, headerBuf)
	n += b
	if err != nil {
		return n, 0, err
	}
	// Bytes 0-3 of the header should equal "OggS"
	if !bytes.Equal(headerBuf[:4:4], []byte{'O', 'g', 'g', 'S'}) {
		return n, 0, fmt.Errorf("invalid oggs header: %q, \"%s\", \"% x\"", headerBuf[:4], headerBuf[:4], headerBuf)
	}
	// Header is valid so read in the data into pageHeader struct
	_, err = header.Read(headerBuf)
	if err != nil {
		return n, 0, err
	}
	// The segment table size must be valid
	if header.Nsegs < 1 {
		return n, 0, errors.New("invalid segment table size")
	}

	// Read in the segment table based on the number of segments we have
	*nsegs = int(header.Nsegs)
	*segTblBuf = d.buffer[HeaderSize : HeaderSize+*nsegs]
	b, err = io.ReadFull(d.src, *segTblBuf)
	n += b
	if err != nil {
		return n, 0, err
	}
	// Calculate the length of the packet data
	var pageDataLen = 0
	for _, l := range *segTblBuf {
		pageDataLen += int(l)
	}
	// Populate the packet buf with the packet data
	*packetBuf = d.buffer[HeaderSize+*nsegs : HeaderSize+*nsegs+pageDataLen]
	b, err = io.ReadFull(d.src, *packetBuf)
	n += b
	if err != nil {
		return n, 0, err
	}

	return n, header.Granule, nil
}

func (d *Decoder) writePage(packetBuf, segTblBuf *[]byte, nsegs *int, dst io.Writer) error {
	// Segment index
	var ixseg int = 0
	// Start and end of our location in
	// the packet buffer which we then
	// write in to the destination buffer
	var start int64 = 0
	var end int64 = 0

	for {
		// Get the full segment size
		for ixseg < *nsegs {
			segment := (*segTblBuf)[ixseg]
			end += int64(segment)

			ixseg++

			if segment < 0xFF {
				break
			}
		}
		// Write the segment to the destination
		_, err := dst.Write((*packetBuf)[start:end])
		if err != nil {
			return fmt.Errorf("failed to write a packet: %w", err)
		}
		// Check if we are done iterating
		if ixseg >= *nsegs {
			return nil
		}
		// Reset the start position to where we are
		start = end
	}
}
