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
	out.Write([]byte(`<!DOCTYPE html><html><body>`))
	defer func() {
		out.Write([]byte(`</body></html>`))
		out.Close()
	}()

	var handle *pcap.Handle

	// Open pcap file
	handle, err = pcap.OpenOffline(inFile)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	var serverPort, sourcePort, lastPort layers.TCPPort = 0, 0, 0

	buf := make([]byte, 0, 2000)

	for packet := range packetSource.Packets() {
		// Get the TCP layer from this packet
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			// Get actual TCP data from this layer
			tcp, _ := tcpLayer.(*layers.TCP)

			if serverPort == 0 && sourcePort == 0 {
				// Assume the first packet is the first SYN
				serverPort, sourcePort = tcp.DstPort, tcp.SrcPort
				lastPort = tcp.SrcPort
			}

			if len(tcp.Payload) == 0 {
				continue
			}

			if lastPort != tcp.SrcPort {
				if len(buf) > 0 {
					fmt.Fprintf(out, "<h5>turn</h5>\n")
					if lastPort == serverPort {
						readServerBytes(buf, out)
					} else {
						readClientBytes(buf, out)
					}
				}

				buf = buf[:0]
				lastPort = tcp.SrcPort
			}

			buf = append(buf, tcp.Payload...)
		}
	}
}

var ignoreClientHandshake int = 5

func readClientBytes(buf []byte, out io.Writer) {
	if ignoreClientHandshake > 0 {
		ignoreClientHandshake -= 1
		fmt.Fprintf(out, "<div>Ignoring %d more handshake packages</div>\n", ignoreClientHandshake)
		return
	}
	offset := 0
	for len(buf) > 0 {
		messageType := buf[0]
		offset = len(buf)
		if messageType == 0 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: Set pixel format to: %02x</div>\n", buf)
		} else if messageType == 2 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: SetEncodings</div>\n")
		} else if messageType == 3 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: FramebufferUpdateRequest</div>\n")
		} else if messageType == 4 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: KeyEvent</div>\n")
		} else if messageType == 5 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: PointerEvent</div>\n")
		} else if messageType == 6 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: ClientCutText</div>\n")
		} else {
			fmt.Fprintf(out, "<div class=\"-error\">Unknown client packet type %d - ignoring all %d bytes</div>\n", messageType, len(buf))
		}
		buf = buf[offset:]
	}
}

var ignoreServerHandshake int = 6

func readServerBytes(buf []byte, out io.Writer) {
	if ignoreServerHandshake > 0 {
		ignoreServerHandshake -= 1
		fmt.Fprintf(out, "<div>Ignoring %d more handshake packages</div>\n", ignoreServerHandshake)
		return
	}
	offset := 0
	for len(buf) > 0 {
		messageType := buf[0]
		offset = len(buf)
		if messageType == 0 {
			offset = decodeFrameBufferUpdate(buf, out)
		} else if messageType == 1 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: SetColourMapEntries</div>\n")
		} else if messageType == 2 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: Bell</div>\n")
		} else if messageType == 3 {
			fmt.Fprintf(out, "<div class=\"-todo\">TODO: ServerCutText</div>\n")
		} else {
			fmt.Fprintf(out, "<div class=\"-error\">Unknown server packet type %d - ignoring all %d bytes</div>\n", messageType, len(buf))
		}
		buf = buf[offset:]
	}
}

func decodeFrameBufferUpdate(buf []byte, out io.Writer) int {
	ppf := ParsePixelFormat([]byte{8, 8, 0, 1, 0, 7, 0, 7, 0, 3, 0, 3, 6, 0, 0, 0})
	targetImage := image.NewRGBA(image.Rect(0, 0, 1280, 720))

	nRects := rInt(buf[2:4])
	log.Printf("Number of rects: %d", nRects)

	offset := 4
	for i := 0; i < nRects; i++ {
		n, img := ppf.nextRect(buf[offset:])
		offset += n

		if img != nil {
			b := img.Bounds()
			draw.Draw(targetImage, b, img, b.Min, draw.Over)
		}
	}

	out.Write([]byte(`<div>framebuffer update<br /><img src="data:image/png;base64,`))
	png.Encode(base64.NewEncoder(base64.StdEncoding, out), targetImage)
	out.Write([]byte(`" /></div>`))

	return offset
}

func (ppf PixelFormat) nextRect(buf []byte) (bytesRead int, img image.Image) {
	x := rInt(buf[0:2])
	y := rInt(buf[2:4])
	w := rInt(buf[4:6])
	h := rInt(buf[6:8])
	enctype := rInt(buf[8:12])
	log.Printf("next rect is a %dx%d rectangle at position %d,%d", w, h, x, y)
	log.Printf("encoding type: %02x (%d)", enctype, enctype)

	rv := image.NewRGBA(image.Rect(x, y, x+w, y+h))

	if enctype == 0 {
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
