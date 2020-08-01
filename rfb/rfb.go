package rfb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

// An RFB represents a captured VNC session
type RFB struct {
	htmlOut      io.WriteCloser
	jsOut        *bytes.Buffer
	clientBuffer *timedBuffer
	serverBuffer *timedBuffer
	timeOffset   float64
	width        int
	height       int
	pixelFormat  PixelFormat
	name         string
}

// New instatiates a new RFB struct
func New(out io.WriteCloser) (*RFB, error) {
	var jsout bytes.Buffer
	var rfb = &RFB{
		htmlOut:      out,
		jsOut:        &jsout,
		clientBuffer: newBuffer(),
		serverBuffer: newBuffer(),
	}

	htmlFragments, err := getAssets(
		"victrola.unpacked.1.html",
		"victrola.unpacked.2.html",
		"victrola.css",
	)
	if err != nil {
		return nil, err
	}

	rfb.htmlOut.Write(htmlFragments[0])

	// Write stuff in the header
	fmt.Fprintf(rfb.htmlOut, "<style>%s</style>\n", htmlFragments[2])

	rfb.htmlOut.Write(htmlFragments[1])

	return rfb, nil
}

// ClientBytes adds a frame of bytes to the Client-side buffer
func (rfb *RFB) ClientBytes(t time.Duration, offset int, buf []byte) error {
	return rfb.clientBuffer.Add(t, offset, buf)
}

// ServerBytes adds a frame of bytes to the Server-side buffer
func (rfb *RFB) ServerBytes(t time.Duration, offset int, buf []byte) error {
	return rfb.serverBuffer.Add(t, offset, buf)
}

func getAssets(names ...string) ([][]byte, error) {
	rv := make([][]byte, len(names))
	var err error
	for i := range names {
		rv[i], err = getAsset(names[i])
		if err != nil {
			return nil, fmt.Errorf("error loading '%s': %s", names[i], err)
		}
	}
	return rv, nil
}

// Close finalizes the replay of the VNC session
func (rfb *RFB) Close() error {
	htmlFragments, err := getAssets("victrola.unpacked.3.html", "victrola.unpacked.4.html")
	if err != nil {
		return err
	}

	playerJS, err := getAsset("player.js")
	if err != nil {
		return err
	}

	defer func() {
		rfb.htmlOut.Write(htmlFragments[1])
		rfb.htmlOut.Close()
	}()

	if err := rfb.consumeHandshake(); err != nil {
		fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
		return err
	}

	fmt.Fprintf(rfb.htmlOut, `<h3>All events</h3>`)
	for rfb.clientBuffer.Remaining() > 0 && rfb.serverBuffer.Remaining() > 0 {
		tC, tS := rfb.clientBuffer.CurrentTime(), rfb.serverBuffer.CurrentTime()

		if rfb.clientBuffer.Remaining() > 0 && tC <= tS {
			if err := rfb.consumeClientEvent(); err != nil {
				fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
				return err
			}
		} else if rfb.serverBuffer.Remaining() > 0 && tS <= tC {
			if err := rfb.consumeServerEvent(); err != nil {
				fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
				return err
			}
		}
	}

	if rfb.clientBuffer.Remaining() > 0 {
		fmt.Fprintf(rfb.htmlOut, `<h3>Client stragglers</h3>`)
		if err := rfb.readAllClientBytes(); err != nil {
			fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
			return err
		}
	}
	if rfb.serverBuffer.Remaining() > 0 {
		fmt.Fprintf(rfb.htmlOut, `<h3>Server stragglers</h3>`)
		if err := rfb.readAllServerBytes(); err != nil {
			fmt.Fprintf(rfb.htmlOut, "<h2>error: %s</h2>\n", err)
			return err
		}
	}

	fmt.Fprintf(rfb.jsOut, "\n\nrfb.Render( document.getElementById('remote-framebuffer-protocol') );\n\n\n")

	rfb.htmlOut.Write(htmlFragments[0])

	fmt.Fprintf(rfb.htmlOut, "<script>")
	rfb.htmlOut.Write(playerJS)
	fmt.Fprintf(rfb.htmlOut, "</script>")
	fmt.Fprintf(rfb.htmlOut, "<script>")
	rfb.jsOut.WriteTo(rfb.htmlOut)
	fmt.Fprintf(rfb.htmlOut, "</script>")

	log.Printf("Replay complete.")
	return nil
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

	// The 'start time' of the replay will be the time at which the final packet in the handshake is sent
	rfb.timeOffset = floatTime(rfb.serverBuffer.CurrentTime())

	// Server init
	sInit := rfb.nextS(24)
	if len(sInit) != 24 {
		return fmt.Errorf("handshake failed: server rejected")
	}
	rfb.width = rInt(sInit[0:2])
	rfb.height = rInt(sInit[2:4])
	rfb.pixelFormat = ParsePixelFormat(sInit[4:20])
	fmt.Fprintf(rfb.htmlOut, "<div>Remote display %dx%d, %s</div>\n", rfb.width, rfb.height, rfb.pixelFormat)
	fmt.Fprintf(rfb.jsOut, "\n\nlet rfb = new RFB( %d, %d );\n\n", rfb.width, rfb.height)
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
	t := floatTime(tEvent)

	var b bytes.Buffer
	var e = json.NewEncoder(&b)
	e.Encode([]interface{}{eventType, t - rfb.timeOffset, eventData})
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

func floatTime(t time.Duration) float64 {
	return float64((int64(t.Microseconds())+50)/100) / 10.0
}
