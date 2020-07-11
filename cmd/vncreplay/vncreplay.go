package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	inFile := "screenshot.dat"
	outFile := "screenshot.png"

	buf, err := ioutil.ReadFile(inFile)
	if err != nil {
		log.Fatal(err)
	}
	out, err := os.Create(outFile)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	ppf := ParsePixelFormat([]byte{8, 8, 0, 1, 0, 7, 0, 7, 0, 3, 0, 3, 6, 0, 0, 0})
	screen := image.NewRGBA(image.Rect(0, 0, 1280, 720))

	ppf.decodeFrameBufferUpdate(buf, screen)

	png.Encode(out, screen)
}

func (ppf PixelFormat) decodeFrameBufferUpdate(buf []byte, targetImage draw.Image) int {
	nRects := rInt(buf[2:4])
	log.Printf("Number of rects: %d", nRects)

	offset := 4
	for i := 0; i < nRects; i++ {
		n, img := ppf.nextRect(buf[offset:])
		offset += n

		b := img.Bounds()
		draw.Draw(targetImage, b, img, b.Min, draw.Over)
	}

	log.Printf("There are %d remaining bytes", len(buf[offset:]))

	return 0
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
				n, c := ppf.ReadPixel(buf[offset:])
				offset += n
				rv.Set(x+i, y+j, c)
			}
		}

		return offset, rv
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
