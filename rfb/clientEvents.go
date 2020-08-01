package rfb

import "fmt"

type keypress struct {
	Key int
}
type pointerupdate struct {
	X, Y                  int
	Lmb, Rmb, Mmb, Su, Sd int
}

func (rfb *RFB) readAllClientBytes() error {
	for rfb.clientBuffer.Remaining() > 0 {
		if err := rfb.consumeClientEvent(); err != nil {
			return err
		}
	}
	return nil
}

func (rfb *RFB) consumeClientEvent() error {
	tEvent := rfb.clientBuffer.CurrentTime()
	messageType := rInt(rfb.clientBuffer.Peek(1))
	if messageType == 0 {
		buf := rfb.nextC(20)
		rfb.pixelFormat = ParsePixelFormat(buf[4:20])
		fmt.Fprintf(rfb.htmlOut, "<div>Pixel format set to: %s</div>\n", rfb.pixelFormat)
	} else if messageType == 2 {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: SetEncodings</div>\n")
		_ = rfb.nextC(2)
		nEncs := rInt(rfb.nextC(2))
		_ = rfb.nextC(4 * nEncs)
	} else if messageType == 3 {
		_ = rfb.nextC(10)
		// fmt.Fprintf(rfb.htmlOut, "<div>Framebuffer Update Request for a %dx%dpx area at %dx%d</div>\n", rInt(buf[2:4]), rInt(buf[4:6]), rInt(buf[6:8]), rInt(buf[8:10]))
	} else if messageType == 4 {
		buf := rfb.nextC(8)
		key := rInt(buf[4:8])
		if rInt(buf[1:2]) == 1 {
			if key < 0x80 {
				fmt.Fprintf(rfb.htmlOut, "<div>Press key 0x%2x (<tt>%c</tt>)</div>\n", key, key)
			} else {
				fmt.Fprintf(rfb.htmlOut, "<div>Press key 0x%2x</div>\n", key)
			}
			rfb.pushEvent("keypress", tEvent, keypress{Key: key})
		} else {
			fmt.Fprintf(rfb.htmlOut, "<div>release key <tt>%c</tt> (0x%2x)</div>\n", key, key)
			rfb.pushEvent("keyrelease", tEvent, keypress{Key: key})
		}
	} else if messageType == 5 {
		buf := rfb.nextC(6)
		bm := rInt(buf[1:2])
		evt := pointerupdate{
			X:   rInt(buf[2:4]),
			Y:   rInt(buf[4:6]),
			Lmb: bm >> 0 & 0x1,
			Rmb: bm >> 1 & 0x1,
			Mmb: bm >> 2 & 0x1,
			Su:  bm >> 3 & 0x1,
			Sd:  bm >> 4 & 0x1,
		}
		fmt.Fprintf(rfb.htmlOut, "<div class=\"pointerupdate\" data-x=\"%d\" data-y=\"%d\" data-bm=\"%d\"></div>\n", evt.X, evt.Y, bm)
		rfb.pushEvent("pointerupdate", tEvent, evt)
	} else if messageType == 6 {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: ClientCutText</div>\n")
		rfb.clientBuffer.Consume(rfb.clientBuffer.Remaining())
	} else if messageType == 111 {
		// Ignore this byte
		rfb.clientBuffer.Consume(1)
	} else {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-error\">Unknown client packet type %d - ignoring all %d bytes</div>\n", messageType, rfb.clientBuffer.Remaining())
		rfb.clientBuffer.Consume(rfb.clientBuffer.Remaining())
	}

	return nil
}
