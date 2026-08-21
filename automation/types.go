package automation

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
)

var (
	ErrUnsupported     = errors.New("automation: unsupported on this platform")
	ErrInvalidRect     = errors.New("automation: invalid rectangle")
	ErrInvalidArgument = errors.New("automation: invalid argument")
	ErrNotFound        = errors.New("automation: not found")
)

// HWND is an opaque native window handle.
type HWND uintptr

// Point is a screen or client coordinate, depending on the operation.
type Point struct{ X, Y int }

// Rect is a half-open rectangle: [Left,Right) x [Top,Bottom).
type Rect struct{ Left, Top, Right, Bottom int }

func (r Rect) Width() int  { return r.Right - r.Left }
func (r Rect) Height() int { return r.Bottom - r.Top }
func (r Rect) Empty() bool { return r.Width() <= 0 || r.Height() <= 0 }
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Left && p.X < r.Right && p.Y >= r.Top && p.Y < r.Bottom
}

func (r Rect) Normalize() Rect {
	if r.Left > r.Right {
		r.Left, r.Right = r.Right, r.Left
	}
	if r.Top > r.Bottom {
		r.Top, r.Bottom = r.Bottom, r.Top
	}
	return r
}

// InclusiveRect converts AHK-style inclusive endpoints to a half-open Rect.
func InclusiveRect(x1, y1, x2, y2 int) Rect {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	return Rect{x1, y1, x2 + 1, y2 + 1}
}

// RGB is an 8-bit red/green/blue color in the same order used by AHK's
// 0xRRGGBB ColorID values.
type RGB struct{ R, G, B uint8 }

func (c RGB) Uint32() uint32     { return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B) }
func RGBFromUint32(v uint32) RGB { return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)} }
func (c RGB) RGBA() color.RGBA   { return color.RGBA{c.R, c.G, c.B, 255} }

// ColorTolerance is applied independently to each channel. AHK's Variation
// maps directly to all three fields having the same value.
type ColorTolerance struct{ R, G, B uint8 }

func Tolerance(v uint8) ColorTolerance { return ColorTolerance{v, v, v} }
func (c RGB) Matches(want RGB, t ColorTolerance) bool {
	abs := func(a, b uint8) uint8 {
		if a > b {
			return a - b
		}
		return b - a
	}
	return abs(c.R, want.R) <= t.R && abs(c.G, want.G) <= t.G && abs(c.B, want.B) <= t.B
}
func abs(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// Bitmap is an immutable snapshot in tightly packed RGBA form.
type Bitmap struct {
	Width, Height int
	Pixels        []byte
}

func NewBitmap(width, height int) (*Bitmap, error) {
	maxInt := int(^uint(0) >> 1)
	if width <= 0 || height <= 0 || width > maxInt/4/height {
		return nil, ErrInvalidRect
	}
	return &Bitmap{Width: width, Height: height, Pixels: make([]byte, width*height*4)}, nil
}
func (b *Bitmap) boundsOK(x, y int) bool {
	if b == nil || x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return false
	}
	i := (int64(y)*int64(b.Width) + int64(x)) * 4
	return i >= 0 && i+3 < int64(len(b.Pixels))
}
func (b *Bitmap) valid() bool {
	return b != nil && b.Width > 0 && b.Height > 0 && int64(b.Width)*int64(b.Height)*4 <= int64(len(b.Pixels))
}
func (b *Bitmap) RGBAt(x, y int) RGB {
	if !b.boundsOK(x, y) {
		return RGB{}
	}
	i64 := (int64(y)*int64(b.Width) + int64(x)) * 4
	if i64 < 0 || i64+3 >= int64(len(b.Pixels)) {
		return RGB{}
	}
	i := int(i64)
	return RGB{b.Pixels[i], b.Pixels[i+1], b.Pixels[i+2]}
}
func (b *Bitmap) ColorModel() color.Model { return color.RGBAModel }
func (b *Bitmap) Bounds() image.Rectangle { return image.Rect(0, 0, b.Width, b.Height) }
func (b *Bitmap) At(x, y int) color.Color {
	if !b.boundsOK(x, y) {
		return color.RGBA{}
	}
	return b.RGBAt(x, y).RGBA()
}
func (b *Bitmap) Set(x, y int, c RGB) {
	if !b.boundsOK(x, y) {
		return
	}
	i64 := (int64(y)*int64(b.Width) + int64(x)) * 4
	if i64 < 0 || i64+3 >= int64(len(b.Pixels)) {
		return
	}
	i := int(i64)
	b.Pixels[i], b.Pixels[i+1], b.Pixels[i+2], b.Pixels[i+3] = c.R, c.G, c.B, 255
}
func (b *Bitmap) Clone() *Bitmap {
	if b == nil {
		return nil
	}
	p := append([]byte(nil), b.Pixels...)
	return &Bitmap{b.Width, b.Height, p}
}
func (b *Bitmap) Crop(r Rect) (*Bitmap, error) {
	if b == nil {
		return nil, ErrInvalidRect
	}
	if !b.valid() {
		return nil, ErrInvalidRect
	}
	if r == (Rect{}) {
		return b.Clone(), nil
	}
	r = r.Normalize()
	if r.Empty() {
		return nil, ErrInvalidRect
	}
	if r.Left < 0 {
		r.Left = 0
	}
	if r.Top < 0 {
		r.Top = 0
	}
	if r.Right > b.Width {
		r.Right = b.Width
	}
	if r.Bottom > b.Height {
		r.Bottom = b.Height
	}
	if r.Empty() {
		return nil, ErrInvalidRect
	}
	out, _ := NewBitmap(r.Width(), r.Height())
	for y := 0; y < r.Height(); y++ {
		copy(out.Pixels[y*out.Width*4:(y+1)*out.Width*4], b.Pixels[((r.Top+y)*b.Width+r.Left)*4:((r.Top+y)*b.Width+r.Right)*4])
	}
	return out, nil
}

type PixelExpectation struct {
	Point     Point
	Color     RGB
	Tolerance ColorTolerance
}

func (b *Bitmap) MatchesAll(expectations ...PixelExpectation) bool {
	for _, e := range expectations {
		if !b.boundsOK(e.Point.X, e.Point.Y) || !b.RGBAt(e.Point.X, e.Point.Y).Matches(e.Color, e.Tolerance) {
			return false
		}
	}
	return true
}
func (b *Bitmap) WritePNG(w io.Writer) error { return png.Encode(w, b) }
func (b *Bitmap) SavePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return b.WritePNG(f)
}
