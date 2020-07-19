package rfb

import (
	"fmt"
	"image/color"
)

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

func (p PixelFormat) ReadPixel(buf []byte) (int, color.RGBA) {
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
