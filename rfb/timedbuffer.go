package rfb

import (
	"log"
	"time"
)

type timedBuffer struct {
	buf   []byte
	index int
}

func newBuffer() *timedBuffer {
	return &timedBuffer{
		buf:   make([]byte, 0),
		index: 0,
	}
}

// Add adds a byte slice to the buffer at offset offset
func (tb *timedBuffer) Add(t time.Duration, offset int, buf []byte) error {
	if offset == len(tb.buf) {
		// Simple case: in-order, contiguous packet delivery
		tb.buf = append(tb.buf, buf...)
	} else if offset > len(tb.buf) {
		// We've skipped some bytes. Fill with skip bytes.
		for i := len(tb.buf); i < offset; i++ {
			tb.buf = append(tb.buf, 111)
		}
		tb.buf = append(tb.buf, buf...)
	} else {
		log.Fatalf("sequence mismatch: already have 0x%02x bytes; about to receive offset 0x%02x", len(tb.buf), offset)
	}
	return nil
}

// Consume returns a slice of l bytes from the buffer, and advances its internal pointer
func (tb *timedBuffer) Consume(l int) []byte {
	start := tb.index
	if (start + l) > len(tb.buf) {
		l = len(tb.buf) - tb.index
	}

	tb.index += l
	return tb.buf[start : start+l]
}

// Peek returns a slice of l bytes from the buffer but does not advance the internal pointer
func (tb *timedBuffer) Peek(l int) []byte {
	if (tb.index + l) > len(tb.buf) {
		l = len(tb.buf) - tb.index
	}

	return tb.buf[tb.index : tb.index+l]
}

// CurrentOffset returns the current value of the internal pointer
func (tb *timedBuffer) CurrentOffset() int {
	return tb.index
}

// Remaining returns the amount of remaining data
func (tb *timedBuffer) Remaining() int {
	return len(tb.buf) - tb.index
}
