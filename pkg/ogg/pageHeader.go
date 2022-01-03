package ogg

import (
	"encoding/binary"
	"io"
)

const (
	// HeaderSize is the header size.
	HeaderSize = 27
	// MaxSegmentSize is the maximum segment size.
	MaxSegmentSize = 255
	// MaxPacketSize is the maximum packet size.
	MaxPacketSize = MaxSegmentSize * 255
	// MaxPageSize is the maximum page size, which is 65307 bytes per the RFC.
	MaxPageSize = HeaderSize + MaxSegmentSize + MaxPacketSize
	// SampleRate is always 48KHz for OPUS
	SampleRate = 48000
)

type pageHeader struct {
	Granule uint64 // For opus, this is the sample position
	Nsegs   byte   // Number of segments in page
}

var byteOrder = binary.LittleEndian

// Read reads b into pageHeader.
func (ph *pageHeader) Read(b []byte) (int, error) {
	if len(b) != HeaderSize {
		return 0, io.ErrUnexpectedEOF
	}

	// We only care about this.
	ph.Granule = byteOrder.Uint64(b[6:14])
	ph.Nsegs = b[26]

	return HeaderSize, nil
}
