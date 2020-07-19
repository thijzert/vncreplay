package rfb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

type RFB struct {
	htmlOut      io.WriteCloser
	jsOut        *bytes.Buffer
	clientBuffer *timedBuffer
	serverBuffer *timedBuffer
	width        int
	height       int
	pixelFormat  PixelFormat
	name         string
}

func New(out io.WriteCloser) *RFB {
	var jsout bytes.Buffer
	var rfb = &RFB{
		htmlOut:      out,
		jsOut:        &jsout,
		clientBuffer: newBuffer(),
		serverBuffer: newBuffer(),
	}

	fmt.Fprintf(rfb.htmlOut, "<!DOCTYPE html>\n<html>\n")
	fmt.Fprintf(rfb.htmlOut, "<head><meta charset=\"UTF-8\"></head>\n")
	fmt.Fprintf(rfb.htmlOut, "<body>\n")
	fmt.Fprintf(rfb.htmlOut, "<div id=\"remote-framebuffer-protocol\"></div>\n")

	return rfb
}

func (rfb *RFB) ClientBytes(t time.Duration, offset int, buf []byte) error {
	return rfb.clientBuffer.Add(t, offset, buf)
}

func (rfb *RFB) ServerBytes(t time.Duration, offset int, buf []byte) error {
	return rfb.serverBuffer.Add(t, offset, buf)
}

func (rfb *RFB) Close() error {
	playerJS, err := getAsset("player.js")
	if err != nil {
		return err
	}

	if err := rfb.consumeHandshake(); err != nil {
		fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
		return err
	}

	fmt.Fprintf(rfb.htmlOut, `<h3>Client events</h3>`)
	if err := rfb.readAllClientBytes(); err != nil {
		fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
		return err
	}

	fmt.Fprintf(rfb.htmlOut, `<h3>Server events</h3>`)
	if err := rfb.readAllServerBytes(); err != nil {
		fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
		return err
	}

	fmt.Fprintf(rfb.jsOut, "\n\nrfb.Render( document.getElementById('remote-framebuffer-protocol') );\n\n\n")

	fmt.Fprintf(rfb.htmlOut, "<script>")
	rfb.htmlOut.Write(playerJS)
	fmt.Fprintf(rfb.htmlOut, "</script>")
	fmt.Fprintf(rfb.htmlOut, "<script>")
	rfb.jsOut.WriteTo(rfb.htmlOut)
	fmt.Fprintf(rfb.htmlOut, "</script>")

	fmt.Fprintf(rfb.htmlOut, `</body></html>`)
	log.Printf("Replay complete.")
	return rfb.htmlOut.Close()
}

func (rfb *RFB) consumeHandshake() error {
	// Server version
	_ = rfb.nextS(12)

	// Client version
	_ = rfb.nextC(12)

	// Server security types
	nSecurity := rInt(rfb.nextS(1))
	_ = rfb.nextS(1 * nSecurity)

	// Client security choice
	sec := rInt(rfb.nextC(1))

	if sec == 2 {
		// VNC authentication
		_ = rfb.nextS(16)
		_ = rfb.nextC(16)
	} else {
		return fmt.Errorf("authentication type %d not implemented", sec)
	}

	// Server init
	securityResult := rInt(rfb.nextS(4))
	if securityResult != 0 {
		return fmt.Errorf("handshake failed: authentication failed: error %d", securityResult)
	}

	// Client init
	cInit := rfb.nextC(1)
	if len(cInit) != 1 {
		return fmt.Errorf("handshake failed: client rejected")
	}

	// Server init
	sInit := rfb.nextS(24)
	if len(sInit) != 24 {
		return fmt.Errorf("handshake failed: server rejected")
	}
	rfb.width = rInt(sInit[0:2])
	rfb.height = rInt(sInit[2:4])
	rfb.pixelFormat = ParsePixelFormat(sInit[4:20])
	fmt.Fprintf(rfb.htmlOut, "<div>Remote display %dx%d, %s</div>\n", rfb.width, rfb.height, rfb.pixelFormat)
	fmt.Fprintf(rfb.jsOut, "\n\nrfb = new RFB( %d, %d );\n\n", rfb.width, rfb.height)
	nlen := rInt(sInit[20:24])
	if nlen > 0 {
		rfb.name = string(rfb.nextS(nlen))
		fmt.Fprintf(rfb.htmlOut, "<div>Server name: %s</div>\n", rfb.name)
	}

	return nil
}

func (rfb *RFB) nextS(l int) []byte {
	return rfb.serverBuffer.Consume(l)
}

func (rfb *RFB) nextC(l int) []byte {
	return rfb.clientBuffer.Consume(l)
}

func (rfb *RFB) pushEvent(eventType string, tEvent time.Duration, eventData interface{}) {

	// Time since start in milliseconds, rounded to 1 decimal
	t := float64((tEvent.Microseconds()+50)/100) / 10.0

	var b bytes.Buffer
	var e = json.NewEncoder(&b)
	e.Encode([]interface{}{eventType, t, eventData})
	s := b.Bytes()

	fmt.Fprintf(rfb.jsOut, "rfb.PushEvent(%s);\n", s[1:len(s)-2])
}

func rInt(b []byte) int {
	var rv int = 0
	for _, c := range b {
		rv = rv<<8 | int(c)
	}
	return rv
}
