package rfb

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
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

type serverCutText struct {
	Text string
}

func (rfb *RFB) readAllServerBytes() error {
	for rfb.serverBuffer.Remaining() > 0 {
		if err := rfb.consumeServerEvent(); err != nil {
			return err
		}
	}
	return nil
}
func (rfb *RFB) consumeServerEvent() error {
	tEvent := rfb.serverBuffer.CurrentTime()
	oldOffset := rfb.serverBuffer.CurrentOffset()
	messageType := rInt(rfb.serverBuffer.Peek(1))
	if messageType == 0 {
		rfb.nextS(rfb.decodeFrameBufferUpdate())
	} else if messageType == 1 {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: SetColourMapEntries</div>\n")
		rfb.serverBuffer.Dump()
	} else if messageType == 2 {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-todo\">TODO: Bell</div>\n")
		rfb.serverBuffer.Dump()
	} else if messageType == 3 {
		buf := rfb.nextS(8)
		cutLen := rInt(buf[4:])
		cutText := string(rfb.nextS(cutLen))
		fmt.Fprintf(rfb.htmlOut, "<div>Server Cut Text: <tt>%s</tt></div>\n", cutText)
		rfb.pushEvent("server-cut-text", tEvent, serverCutText{Text: cutText})
	} else if messageType == 111 {
		// Ignore this byte
		rfb.serverBuffer.Consume(1)
	} else {
		fmt.Fprintf(rfb.htmlOut, "<div class=\"-error\">Unknown server packet type %d at offset %8x - ignoring all %d bytes</div>\n", messageType, rfb.serverBuffer.CurrentOffset(), rfb.serverBuffer.Remaining())
		rfb.serverBuffer.Dump()
	}
	if messageType != 111 {
		length := rfb.serverBuffer.CurrentOffset() - oldOffset
		log.Printf("Server packet of type %d consumed at index %08x (%d) len %d - next packet at %08x", messageType, oldOffset, oldOffset, length, rfb.serverBuffer.CurrentOffset())
	}

	return nil
}

type encodedImage struct {
	Bounds   image.Rectangle
	Mimetype string
	Contents []byte
}

func encodeImage(img image.Image) encodedImage {
	var b bytes.Buffer
	err := png.Encode(&b, img)
	if err != nil {
		log.Printf("Error encoding rect as PNG: %v", err)
	}

	return encodedImage{
		Bounds:   img.Bounds(),
		Mimetype: "image/png",
		Contents: b.Bytes(),
	}
}

func (rfb *RFB) decodeFrameBufferUpdate() int {
	tEvent := rfb.serverBuffer.CurrentTime()
	buf := rfb.serverBuffer.Peek(rfb.serverBuffer.Remaining())
	nRects := rInt(buf[2:4])
	log.Printf("Number of rects: %d", nRects)
	var rects []encodedImage

	offset := 4
	for i := 0; i < nRects; i++ {
		n, img, enctype := rfb.nextRect(buf[offset:])
		offset += n

		if enctype == -239 {
			rfb.handleCursorUpdate(img)
		} else if len(img.Contents) > 0 {
			rects = append(rects, img)
		}
	}

	if len(rects) > 0 {
		fmt.Fprintf(rfb.htmlOut, "<div id=\"framebuffer_%08x\">framebuffer update:", rfb.serverBuffer.CurrentOffset())

		for _, img := range rects {
			fmt.Fprintf(rfb.htmlOut, "<br />\n\t<img style=\"maxx-width: 7.5em;\" data-x=\"%d\" data-y=\"%d\" src=\"data:%s;base64,", img.Bounds.Min.X, img.Bounds.Min.Y, img.Mimetype)
			e := base64.NewEncoder(base64.StdEncoding, rfb.htmlOut)
			e.Write(img.Contents)
			e.Close()
			fmt.Fprintf(rfb.htmlOut, "\">")
		}
		fmt.Fprintf(rfb.htmlOut, "\n</div>")

		rfb.pushEvent("framebuffer", tEvent, framebuffer{Id: fmt.Sprintf("framebuffer_%08x", rfb.serverBuffer.CurrentOffset())})
	}

	return offset
}

func (rfb *RFB) handleCursorUpdate(img encodedImage) {
	tEvent := rfb.serverBuffer.CurrentTime()
	if img.Bounds.Dx() > 0 && img.Bounds.Dy() > 0 {
		min := img.Bounds.Min
		fmt.Fprintf(rfb.htmlOut, `<div>Draw cursor like this: <img id="pointer_%08x" src="data:image/png;base64,`, rfb.serverBuffer.CurrentOffset())
		f := base64.NewEncoder(base64.StdEncoding, rfb.htmlOut)
		f.Write(img.Contents)
		f.Close()
		fmt.Fprintf(rfb.htmlOut, "\" /></div>\n")

		rfb.pushEvent("pointer-skin", tEvent, pointerSkin{
			Id: fmt.Sprintf("pointer_%08x", rfb.serverBuffer.CurrentOffset()),
			X:  min.X,
			Y:  min.Y,
		})
	} else {
		fmt.Fprintf(rfb.htmlOut, "<div>Use the default cursor from here.</div>\n")
		rfb.pushEvent("pointer-skin", tEvent, pointerSkin{Default: 1})
	}
}

func (rfb *RFB) nextRect(buf []byte) (bytesRead int, img encodedImage, enctype int32) {
	x := rInt(buf[0:2])
	y := rInt(buf[2:4])
	w := rInt(buf[4:6])
	h := rInt(buf[6:8])
	enctype = int32(uint32(rInt(buf[8:12])))
	log.Printf("next rect is a %dx%d rectangle at position %d,%d enctype %02x (%d)", w, h, x, y, enctype, enctype)

	rv := image.NewRGBA(image.Rect(x, y, x+w, y+h))

	if enctype == 0 || enctype == -239 {
		// Raw encoding

		offset := 12
		rectEnd := 12 + h*w*rfb.pixelFormat.BytesPerPixel()
		bitmaskOffset := 0
		if enctype == -239 {
			lineLength := (w + 7) / 8
			bitmaskOffset = h * lineLength
		}
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				if offset >= len(buf) {
					// log.Printf("Warning: image truncated")
					return offset, encodeImage(rv), enctype
				}

				n, c := rfb.pixelFormat.ReadPixel(buf[offset:])
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

		return offset + bitmaskOffset, encodeImage(rv), enctype
	} else if enctype == -232 {
		// Pointer pos (pseudo)
	} else if enctype == 7 {
		// TightVNC
		// This mode supports zlib encoding across packets, and is the part that
		// makes this pixel decoder stateful.

		compressionControl := buf[12]
		streamMask := compressionControl & 0x0f
		for i := 0; i < 4; i++ {
			if (streamMask>>(3-i))&1 == 1 {
				rfb.zlibStreams[i] = nil
				rfb.zlibInputs[i].Truncate(0)
			}
		}
		streamID := int(compressionControl >> 4 & 0x3)

		if compressionControl>>4 == 9 {
			// JPEG
			rectLength, rectLengthLength := compactLength(buf[13:16])
			eimg := encodedImage{
				Bounds:   rv.Bounds(),
				Mimetype: "image/jpeg",
				Contents: buf[13+rectLengthLength : 13+rectLengthLength+rectLength],
			}
			log.Printf("got %d bytes of JPEG data", rectLength)
			return 13 + rectLengthLength + rectLength, eimg, enctype
		} else if compressionControl>>4 == 8 {
			// Rect fill
			fillColour := color.RGBA{buf[13], buf[14], buf[15], 255}
			log.Printf("solid fill with %02x%02x%02x", fillColour.R, fillColour.G, fillColour.B)

			for b := y; b < y+h; b++ {
				for a := x; a < x+w; a++ {
					rv.Set(a, b, fillColour)
				}
			}
			return 16, encodeImage(rv), enctype
		} else if compressionControl&0xc0 == 0x40 && buf[13] == 1 {
			// Basic/Paletted
			paletteLength := int(buf[14]) + 1
			var pixbuf []byte
			if paletteLength <= 2 {
				pixbuf = make([]byte, h*((w+7)/8))
			} else {
				pixbuf = make([]byte, h*w)
			}
			log.Printf("5 %d colours in palette; stream ID %d", paletteLength, streamID)

			rectLength, rectLengthLength := 0, 0
			if len(pixbuf) < 12 {
				// buffer is too small to compress - copy it exactly
				rectLength, rectLengthLength = len(pixbuf), 0
				copy(pixbuf, buf)
			} else {
				rectLength, rectLengthLength = compactLength(buf[15+3*paletteLength:])
				ioff := 15 + 3*paletteLength + rectLengthLength
				nn, err := rfb.readZlib(streamID, pixbuf, buf[ioff:ioff+rectLength])
				if err != nil || len(pixbuf) > nn {
					log.Printf("error reading from zlib stream; only read %d bytes: %v", nn, err)
				}
			}
			log.Printf("5 %d colours in palette; %d bytes compressed image data, %d=%dx%d pixels", paletteLength, rectLength, w*h, w, h)

			palette := make([]color.Color, paletteLength)
			for i := range palette {
				palette[i] = color.RGBA{
					R: buf[3*i+15],
					G: buf[3*i+16],
					B: buf[3*i+17],
					A: 255,
				}
			}

			if paletteLength < 2 {
				return 15 + 3*paletteLength + rectLengthLength + rectLength, encodedImage{}, enctype
			} else if paletteLength == 2 {
				for b := 0; b < h; b++ {
					hoff := b * ((w + 7) >> 3)
					for a := 0; a < w; a++ {
						rv.Set(x+a, y+b, palette[int(pixbuf[hoff+(a>>3)]>>(7-(a&0x7)))%1])
					}
				}
			} else {
				for b := 0; b < h; b++ {
					for a := 0; a < w; a++ {
						rv.Set(x+a, y+b, palette[int(pixbuf[b*w+a])%len(palette)])
					}
				}
			}

			return 15 + 3*paletteLength + rectLengthLength + rectLength, encodeImage(rv), enctype
		} else {
			log.Printf("unknown compression control byte %02x - ignoring rest of buffer", compressionControl)
			return len(buf), encodedImage{}, enctype
		}
	} else {
		log.Printf("Unknown encoding type %d - ignoring whole buffer", enctype)
		return len(buf), encodedImage{}, enctype
	}

	return 12, encodedImage{}, 0
}

func (rfb *RFB) readZlib(streamID int, dest []byte, src []byte) (int, error) {
	log.Printf("Writing to zlib reader %d: %02x", streamID, src)
	rfb.zlibInputs[streamID].Write(src)
	if rfb.zlibStreams[streamID] == nil {
		zs, err := zlib.NewReader(&rfb.zlibInputs[streamID])
		if err != nil {
			return 0, fmt.Errorf("cannot initialise zlib stream: %w", err)
		}
		rfb.zlibStreams[streamID] = zs
	}

	var nn int
	for len(dest[nn:]) > 0 {
		n, err := rfb.zlibStreams[streamID].Read(dest[nn:])
		nn += n
		if err != nil {
			return nn, err
		}
	}
	return nn, nil
}

func compactLength(buf []byte) (length int, lengthLength int) {
	if len(buf) > 0 && buf[0]>>7 == 0 {
		return int(buf[0] & 0x7f), 1
	} else if len(buf) > 1 && buf[0]>>7 == 1 && buf[1]>>7 == 0 {
		return int(buf[1]&0x7f)<<7 | int(buf[0]&0x7f), 2
	} else if len(buf) > 2 && buf[0]>>7 == 1 && buf[1]>>7 == 1 {
		return int(buf[2])<<14 | int(buf[1]&0x7f)<<7 | int(buf[0]&0x7f), 3
	}
	return len(buf), 8
}
