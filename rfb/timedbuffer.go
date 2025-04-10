package rfb

import (
	"fmt"
	"log"
	"time"
)

type timeindex struct {
	t time.Duration
	i int
}

type timedBuffer struct {
	buf         []byte
	timingDirty bool
	timing      []timeindex
	tmax        time.Duration
	index       int
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
		tb.timing = append(tb.timing, timeindex{t, len(tb.buf)})
		tb.buf = append(tb.buf, buf...)
	} else if offset > len(tb.buf) {
		// We've skipped some bytes. Fill with skip bytes.
		for i := len(tb.buf); i < offset; i++ {
			tb.buf = append(tb.buf, 111)
		}
		tb.timing = append(tb.timing, timeindex{t, len(tb.buf)})
		tb.buf = append(tb.buf, buf...)
	} else {
		// TCP retransmission, or out of order delivery
		if len(tb.buf[offset:]) < len(buf) {
			tb.timingDirty = true
			return fmt.Errorf("sequence mismatch: already have 0x%02x bytes; about to receive offset 0x%02x (dealing with this has not been implemented)", len(tb.buf), offset)
		}
		copy(tb.buf[offset:], buf)
	}

	if t > tb.tmax {
		tb.tmax = t
	}

	return nil
}

// Consume returns a slice of l bytes from the buffer, and advances its
// internal pointer
func (tb *timedBuffer) Consume(l int) []byte {
	start := tb.index
	if (start + l) > len(tb.buf) {
		l = len(tb.buf) - tb.index
	}

	tb.index += l
	return tb.buf[start : start+l]
}

// Dump discards bytes from the buffer until the next logical start point, or
// until the end, whichever comes first.
func (tb *timedBuffer) Dump() int {
	rv := tb.Remaining()

	// Try to find the next packet boundary
	if !tb.timingDirty {
		for _, tc := range tb.timing {
			if tc.i <= tb.index {
				continue
			}
			rv = tc.i - tb.index
			break
		}
	}

	tb.index += rv
	return rv
}

// Peek returns a slice of l bytes from the buffer but does not advance the
// internal pointer
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

// CurrentTime returns the approximate timing of the next byte at the internal
// pointer
func (tb *timedBuffer) CurrentTime() time.Duration {
	if tb.timingDirty {
		log.Fatalf("Todo: sorting")
	}
	var rv time.Duration
	for _, tc := range tb.timing {
		if tc.i <= tb.index {
			rv = tc.t
		} else {
			return rv
		}
	}

	return tb.tmax + 1*time.Millisecond
}

// Remaining returns the amount of remaining data
func (tb *timedBuffer) Remaining() int {
	return len(tb.buf) - tb.index
}
