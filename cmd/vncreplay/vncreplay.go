package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func main() {
	var inFile, outFile string
	flag.StringVar(&inFile, "i", "", "Input file")
	flag.StringVar(&outFile, "o", "replay.html", "Output file")
	flag.Parse()

	if inFile == "" {
		if len(flag.Args()) > 0 {
			inFile = flag.Args()[0]
		} else {
			log.Fatalf("Usage: replay [-o OUTFILE] INFILE")
		}
	}

	out, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	rfb := NewRFB(out)
	defer rfb.Close()

	var handle *pcap.Handle

	// Open pcap file
	handle, err = pcap.OpenOffline(inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	var serverPort, sourcePort layers.TCPPort = 0, 0

	for packet := range packetSource.Packets() {
		// Get the TCP layer from this packet
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			// Get actual TCP data from this layer
			tcp, _ := tcpLayer.(*layers.TCP)

			if serverPort == 0 && sourcePort == 0 {
				// Assume the first packet is the first SYN
				serverPort, sourcePort = tcp.DstPort, tcp.SrcPort
			}

			if len(tcp.Payload) == 0 {
				continue
			}

			err = nil
			if tcp.SrcPort == serverPort {
				err = rfb.ServerBytes(tcp.Payload)
			} else if tcp.SrcPort == sourcePort {
				err = rfb.ClientBytes(tcp.Payload)
			} else {
				log.Printf("Ignoring extra traffic")
			}
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

type RFBState int

const (
	StateUninitialised RFBState = iota
	StateServerVersionSent
	StateClientVersionSent
	StateSecurityOffered
	StateSecurityChosen
	StateSecurityChallengeSent
	StateSecurityChallengeAccepted
	StateSecurityOK
	StateClientInitSent
	StateServerInitSent
	StateClientTalking
	StateServerTalking
)

func (s RFBState) String() string {
	return []string{
		"uninitialised",
		"server version sent",
		"client version sent",
		"security offered",
		"security chosen",
		"security challenge sent",
		"security challenge accepted",
		"security ok",
		"client init sent",
		"server init sent",
		"client talking",
		"server talking",
	}[int(s)]
}

type RFB struct {
	state        RFBState
	htmlOut      io.WriteCloser
	clientBuffer []byte
	serverBuffer []byte
	width        int
	height       int
	pixelFormat  PixelFormat
	name         string
}

func NewRFB(out io.WriteCloser) *RFB {
	var rfb = &RFB{
		state:        StateUninitialised,
		htmlOut:      out,
		clientBuffer: make([]byte, 0, 2000),
		serverBuffer: make([]byte, 0, 2000),
	}

	rfb.htmlOut.Write([]byte(`<!DOCTYPE html><html><body>`))

	return rfb
}

func (rfb *RFB) Close() error {
	rfb.htmlOut.Write([]byte(`</body></html>`))
	return rfb.htmlOut.Close()
}

func (rfb *RFB) ClientBytes(buf []byte) error {
	if rfb.state == StateServerTalking {
		rfb.readServerBytes()
		rfb.clientBuffer = append(rfb.clientBuffer, buf...)
		rfb.state = StateClientTalking
	} else if rfb.state == StateClientTalking {
		rfb.clientBuffer = append(rfb.clientBuffer, buf...)
	} else if rfb.state == StateServerVersionSent {
		rfb.state = StateClientVersionSent
	} else if rfb.state == StateSecurityOffered {
		rfb.state = StateSecurityChosen
	} else if rfb.state == StateSecurityChallengeSent {
		rfb.state = StateSecurityChallengeAccepted
	} else if rfb.state == StateSecurityOK {
		rfb.state = StateClientInitSent
	} else if rfb.state == StateServerInitSent {
		rfb.clientBuffer = append(rfb.clientBuffer, buf...)
		rfb.state = StateClientTalking
	} else {
		return fmt.Errorf("handshake error: no client bytes expected in state '%s'", rfb.state)
	}
	return nil
}

func (rfb *RFB) ServerBytes(buf []byte) error {
	if rfb.state == StateServerTalking {
		rfb.serverBuffer = append(rfb.serverBuffer, buf...)
	} else if rfb.state == StateClientTalking {
		rfb.readClientBytes()
		rfb.serverBuffer = append(rfb.serverBuffer, buf...)
		rfb.state = StateServerTalking
	} else if rfb.state == StateUninitialised {
		rfb.state = StateServerVersionSent
	} else if rfb.state == StateClientVersionSent {
		rfb.state = StateSecurityOffered
	} else if rfb.state == StateSecurityChosen {
		rfb.state = StateSecurityChallengeSent
	} else if rfb.state == StateSecurityChallengeAccepted {
		rfb.state = StateSecurityOK
	} else if rfb.state == StateClientInitSent {
		rfb.state = StateServerInitSent
		rfb.width = rInt(buf[0:2])
		rfb.height = rInt(buf[2:4])
		rfb.pixelFormat = ParsePixelFormat(buf[4:20])
		fmt.Fprintf(rfb.htmlOut, "<div>Remote display %dx%d, %s</div>\n", rfb.width, rfb.height, rfb.pixelFormat)
		nlen := rInt(buf[20:24])
		if nlen > 0 {
			rfb.name = string(buf[24 : 24+nlen])
			fmt.Fprintf(rfb.htmlOut, "<div>Server name: %s</div>\n", rfb.name)
		}
	} else if rfb.state == StateServerInitSent {
		rfb.serverBuffer = append(rfb.serverBuffer, buf...)
		rfb.state = StateServerTalking
	} else {
		return fmt.Errorf("handshake error: no server bytes expected in state '%s'", rfb.state)
	}
	return nil
}

func (rfb *RFB) readClientBytes() {
	offset := 0
	for len(rfb.clientBuffer) > 0 {
		messageType := rfb.clientBuffer[0]
		offset = len(rfb.clientBuffer)
		if messageType == 0 {
			rfb.pixelFormat = ParsePixelFormat(rfb.clientBuffer[4:20])
			fmt.Fprintf(rfb.htmlOut, "<div>Pixel format set to: %s</div>\n", rfb.pixelFormat)
			offset = 20
		} else if messageType == 2 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: SetEncodings</div>\n")
			offset = 4 + rInt(rfb.clientBuffer[2:4])*4
		} else if messageType == 3 {
			offset = 10
			fmt.Fprintf(rfb.htmlOut, "<div>Framebuffer Update Request for a %dx%dpx area at %dx%d</div>\n", rInt(rfb.clientBuffer[2:4]), rInt(rfb.clientBuffer[4:6]), rInt(rfb.clientBuffer[6:8]), rInt(rfb.clientBuffer[8:10]))
		} else if messageType == 4 {
			key := rInt(rfb.clientBuffer[4:8])
			if rInt(rfb.clientBuffer[1:2]) == 1 {
				fmt.Fprintf(rfb.htmlOut, "<div>Press key <tt>%c</tt> (0x%2x)</div>\n", key, key)
			} else {
				fmt.Fprintf(rfb.htmlOut, "<div>release key <tt>%c</tt> (0x%2x)</div>\n", key, key)
			}
			offset = 8
		} else if messageType == 5 {
			bm := rInt(rfb.clientBuffer[1:2])
			x := rInt(rfb.clientBuffer[2:4])
			y := rInt(rfb.clientBuffer[4:6])
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">Move pointer to %d,%d with buttons %x</div>\n", x, y, bm)
			offset = 6
		} else if messageType == 6 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: ClientCutText</div>\n")
		} else {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-error\">Unknown client packet type %d - ignoring all %d bytes</div>\n", messageType, len(rfb.clientBuffer))
		}
		rfb.clientBuffer = rfb.clientBuffer[offset:]
	}
}

func (rfb *RFB) readServerBytes() {
	offset := 0
	for len(rfb.serverBuffer) > 0 {
		messageType := rfb.serverBuffer[0]
		offset = len(rfb.serverBuffer)
		if messageType == 0 {
			offset = rfb.decodeFrameBufferUpdate()
		} else if messageType == 1 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: SetColourMapEntries</div>\n")
		} else if messageType == 2 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: Bell</div>\n")
		} else if messageType == 3 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: ServerCutText</div>\n")
		} else {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-error\">Unknown server packet type %d - ignoring all %d bytes</div>\n", messageType, len(rfb.serverBuffer))
		}
		rfb.serverBuffer = rfb.serverBuffer[offset:]
	}
}

func (rfb *RFB) decodeFrameBufferUpdate() int {
	targetImage := image.NewRGBA(image.Rect(0, 0, rfb.width, rfb.height))

	nRects := rInt(rfb.serverBuffer[2:4])
	log.Printf("Number of rects: %d", nRects)

	offset := 4
	for i := 0; i < nRects; i++ {
		n, img := rfb.pixelFormat.nextRect(rfb.serverBuffer[offset:])
		offset += n

		if img != nil {
			b := img.Bounds()
			draw.Draw(targetImage, b, img, b.Min, draw.Over)
		}
	}

	rfb.htmlOut.Write([]byte(`<div>framebuffer update<br /><img src="data:image/png;base64,`))
	png.Encode(base64.NewEncoder(base64.StdEncoding, rfb.htmlOut), targetImage)
	rfb.htmlOut.Write([]byte(`" /></div>`))

	return offset
}

func (ppf PixelFormat) nextRect(buf []byte) (bytesRead int, img image.Image) {
	x := rInt(buf[0:2])
	y := rInt(buf[2:4])
	w := rInt(buf[4:6])
	h := rInt(buf[6:8])
	enctype := int32(uint32(rInt(buf[8:12])))
	log.Printf("next rect is a %dx%d rectangle at position %d,%d", w, h, x, y)
	log.Printf("encoding type: %02x (%d)", enctype, enctype)

	rv := image.NewRGBA(image.Rect(x, y, x+w, y+h))

	if enctype == 0 || enctype == -239 {
		// Raw encoding

		offset := 12
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				if offset >= len(buf) {
					log.Printf("Warning: image truncated")
					return offset, rv
				}

				n, c := ppf.ReadPixel(buf[offset:])
				offset += n
				rv.Set(x+i, y+j, c)
			}
		}

		return offset, rv
	} else {
		log.Printf("Unknown encoding type - ignoring whole buffer")
		return len(buf), nil
	}

	return 12, nil
}

func rInt(b []byte) int {
	var rv int = 0
	for _, c := range b {
		rv = rv<<8 | int(c)
	}
	return rv
}

type PixelFormat struct {
	Bits       int
	Depth      int
	BigEndian  bool
	TrueColour bool
	RedMax     uint
	GreenMax   uint
	BlueMax    uint
	RedShift   uint
	GreenShift uint
	BlueShift  uint
}

func ParsePixelFormat(buf []byte) PixelFormat {
	var rv PixelFormat
	rv.Bits = rInt(buf[0:1])
	rv.Depth = rInt(buf[1:2])
	rv.BigEndian = rInt(buf[2:3]) > 0
	rv.TrueColour = rInt(buf[3:4]) > 0
	rv.RedMax = uint(rInt(buf[4:6]))
	rv.GreenMax = uint(rInt(buf[6:8]))
	rv.BlueMax = uint(rInt(buf[8:10]))
	rv.RedShift = uint(rInt(buf[10:11]))
	rv.GreenShift = uint(rInt(buf[11:12]))
	rv.BlueShift = uint(rInt(buf[12:13]))
	return rv
}

func (p PixelFormat) BytesPerPixel() int {
	return (p.Bits + 7) / 8
}

func (p PixelFormat) ReadPixel(buf []byte) (int, color.Color) {
	l := (p.Bits + 7) / 8
	var pixel uint = 0
	if l == 1 {
		pixel = uint(buf[0])
	} else if p.BigEndian {
		for i := 0; i < l; i++ {
			pixel = pixel<<8 | uint(buf[0])
		}
	} else {
		for i := 0; i < l; i++ {
			pixel = pixel<<8 | uint(buf[l-i-1])
		}
	}

	r := (pixel >> p.RedShift) & p.RedMax
	g := (pixel >> p.GreenShift) & p.GreenMax
	b := (pixel >> p.BlueShift) & p.BlueMax

	return l, color.RGBA{
		R: uint8((r * 0xff) / p.RedMax),
		G: uint8((g * 0xff) / p.GreenMax),
		B: uint8((b * 0xff) / p.BlueMax),
		A: 0xff,
	}
}

func (p PixelFormat) String() string {
	if p.TrueColour {
		return fmt.Sprintf("%d-bit true colour", p.Bits)
	} else {
		return fmt.Sprintf("%d-bit mapped", p.Bits)
	}
}
