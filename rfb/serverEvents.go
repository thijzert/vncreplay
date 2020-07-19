package rfb

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
)

type framebuffer struct {
	Id string
}

type pointerSkin struct {
	Id      string
	Default int
	X, Y    int
}

func (rfb *RFB) readServerBytes() error {
	for len(rfb.serverBuffer) > rfb.serverOffset {
		oldOffset := rfb.serverOffset
		messageType := rfb.serverBuffer[rfb.serverOffset]
		if messageType == 0 {
			rfb.nextS(rfb.decodeFrameBufferUpdate())
		} else if messageType == 1 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: SetColourMapEntries</div>\n")
			rfb.nextS(len(rfb.serverBuffer))
		} else if messageType == 2 {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: Bell</div>\n")
			rfb.nextS(len(rfb.serverBuffer))
		} else if messageType == 3 {
			buf := rfb.nextS(8)
			cutLen := rInt(buf[4:])
			cutText := rfb.nextS(cutLen)
			fmt.Fprintf(rfb.htmlOut, "<div>Server Cut Text: <tt>%s</tt></div>\n", cutText)
		} else if messageType == 111 {
			// Ignore this byte
			rfb.nextS(1)
		} else {
			fmt.Fprintf(rfb.htmlOut, "<div class=\"-error\">Unknown server packet type %d at offset %8x - ignoring all %d bytes</div>\n", messageType, rfb.serverOffset, rfb.serverOffset, len(rfb.serverBuffer[rfb.serverOffset:]))
			rfb.nextS(len(rfb.serverBuffer))
		}
		if messageType != 111 {
			length := rfb.serverOffset - oldOffset
			log.Printf("Server packet of type %d consumed at index %08x (%d) len %d - next packet at %08x", messageType, oldOffset, oldOffset, length, rfb.serverOffset)
		}
	}

	return nil
}

func (rfb *RFB) decodeFrameBufferUpdate() int {
	targetImage := image.NewRGBA(image.Rect(0, 0, rfb.width, rfb.height))

	nRects := rInt(rfb.serverBuffer[rfb.serverOffset+2 : rfb.serverOffset+4])
	rectsAdded := 0
	// log.Printf("Number of rects: %d", nRects)

	offset := 4
	for i := 0; i < nRects; i++ {
		n, img, enctype := rfb.pixelFormat.nextRect(rfb.serverBuffer[rfb.serverOffset+offset:])
		offset += n

		if enctype == -239 {
			rfb.handleCursorUpdate(img)
		} else if img != nil {
			b := img.Bounds()
			draw.Draw(targetImage, b, img, b.Min, draw.Over)
			rectsAdded++
		}
	}

	if rectsAdded > 0 {
		fmt.Fprintf(rfb.htmlOut, "<div>framebuffer update: <img style=\"max-width: 1.5em;\" id=\"framebuffer_%08x\" src=\"data:image/png;base64,", rfb.serverOffset)
		png.Encode(base64.NewEncoder(base64.StdEncoding, rfb.htmlOut), targetImage)
		fmt.Fprintf(rfb.htmlOut, "\" /></div>\n")

		rfb.pushEvent("framebuffer", framebuffer{Id: fmt.Sprintf("framebuffer_%08x", rfb.serverOffset)})
	}

	return offset
}

func (rfb *RFB) handleCursorUpdate(img image.Image) {
	if img.Bounds().Dx() > 0 && img.Bounds().Dy() > 0 {
		min := img.Bounds().Min
		fmt.Fprintf(rfb.htmlOut, `<div>Draw cursor like this: <img id="pointer_%08x" src="data:image/png;base64,`, rfb.serverOffset)
		png.Encode(base64.NewEncoder(base64.StdEncoding, rfb.htmlOut), img)
		fmt.Fprintf(rfb.htmlOut, "\" /></div>\n")

		rfb.pushEvent("pointer-skin", pointerSkin{
			Id: fmt.Sprintf("pointer_%08x", rfb.serverOffset),
			X:  min.X,
			Y:  min.Y,
		})
	} else {
		fmt.Fprintf(rfb.htmlOut, "<div>Use the default cursor from here.</div>\n")
		rfb.pushEvent("pointer-skin", pointerSkin{Default: 1})
	}
}

func (ppf PixelFormat) nextRect(buf []byte) (bytesRead int, img image.Image, enctype int32) {
	x := rInt(buf[0:2])
	y := rInt(buf[2:4])
	w := rInt(buf[4:6])
	h := rInt(buf[6:8])
	enctype = int32(uint32(rInt(buf[8:12])))
	// log.Printf("next rect is a %dx%d rectangle at position %d,%d enctype %02x (%d)", w, h, x, y, enctype, enctype)

	rv := image.NewRGBA(image.Rect(x, y, x+w, y+h))

	if enctype == 0 || enctype == -239 {
		// Raw encoding

		offset := 12
		rectEnd := 12 + h*w*ppf.BytesPerPixel()
		bitmaskOffset := 0
		if enctype == -239 {
			lineLength := (w + 7) / 8
			bitmaskOffset = h * lineLength
		}
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				if offset >= len(buf) {
					// log.Printf("Warning: image truncated")
					return offset, rv, enctype
				}

				n, c := ppf.ReadPixel(buf[offset:])
				offset += n

				if enctype == -239 {
					// The cursor update pseudoformat also consists of a bitmask after
					// the pixel colours, corresponding to the alpha value of each pixel.

					lineLength := (w + 7) / 8
					aByte := j*lineLength + i/8
					aBit := i & 0x7
					if len(buf) > rectEnd+aByte {
						if (buf[rectEnd+aByte]<<aBit)&0x80 == 0 {
							c.A = 0
						}
					}
				}

				rv.Set(x+i, y+j, c)
			}
		}

		return offset + bitmaskOffset, rv, enctype
	} else {
		log.Printf("Unknown encoding type - ignoring whole buffer")
		return len(buf), nil, enctype
	}

	return 12, nil, 0
}
