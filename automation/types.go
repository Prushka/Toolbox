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
	ErrMonitorActive   = errors.New("automation: input monitor already active")
	ErrMonitorClosed   = errors.New("automation: input monitor closed")
)

// HWND is an opaque native window handle.
type HWND uintptr

// Point is a screen or client coordinate, depending on the operation.
type Point struct{ X, Y int }

// Rect is a half-open rectangle: [Left,Right) x [Top,Bottom).
type Rect struct{ Left, Top, Right, Bottom int }

var (
	maxInt = int(^uint(0) >> 1)
	minInt = -maxInt - 1
)

func checkedAddInt(a, b int) (int, bool) {
	if (b > 0 && a > maxInt-b) || (b < 0 && a < minInt-b) {
		return 0, false
	}
	return a + b, true
}

func checkedSubInt(a, b int) (int, bool) {
	if (b > 0 && a < minInt+b) || (b < 0 && a > maxInt+b) {
		return 0, false
	}
	return a - b, true
}

func checkedMulInt(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == minInt && b == -1) || (b == minInt && a == -1) {
		return 0, false
	}
	v := a * b
	return v, v/b == a
}

func (r Rect) Width() int {
	v, ok := checkedSubInt(r.Right, r.Left)
	if !ok {
		return 0
	}
	return v
}
func (r Rect) Height() int {
	v, ok := checkedSubInt(r.Bottom, r.Top)
	if !ok {
		return 0
	}
	return v
}
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
	right, bottom := x2, y2
	if right < maxInt {
		right++
	}
	if bottom < maxInt {
		bottom++
	}
	return Rect{x1, y1, right, bottom}
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
	return abs(c.R, want.R) <= t.R && abs(c.G, want.G) <= t.G && abs(c.B, want.B) <= t.B
}
func abs(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// Bitmap stores tightly packed RGBA pixels. The alpha byte is storage padding:
// Bitmap image operations and PNG output are always opaque. Reads and writes
// are safe only when callers do not mutate the bitmap concurrently.
type Bitmap struct {
	Width, Height int
	Pixels        []byte
}

const maxBitmapBytes = 512 << 20

func NewBitmap(width, height int) (*Bitmap, error) {
	bytes, ok := bitmapByteLen(width, height)
	if !ok || bytes > maxBitmapBytes {
		return nil, ErrInvalidRect
	}
	return &Bitmap{Width: width, Height: height, Pixels: make([]byte, bytes)}, nil
}

func bitmapByteLen(width, height int) (int, bool) {
	if width <= 0 || height <= 0 {
		return 0, false
	}
	if width > maxInt/4/height {
		return 0, false
	}
	return width * height * 4, true
}

func (b *Bitmap) boundsOK(x, y int) bool {
	if b == nil || x < 0 || y < 0 || x >= b.Width || y >= b.Height {
		return false
	}
	row, ok := checkedMulInt(y, b.Width)
	if !ok || x > maxInt-row {
		return false
	}
	pixel := row + x
	return pixel <= maxInt/4 && pixel < len(b.Pixels)/4
}
func (b *Bitmap) valid() bool {
	if b == nil {
		return false
	}
	bytes, ok := bitmapByteLen(b.Width, b.Height)
	return ok && bytes <= len(b.Pixels) && bytes <= maxBitmapBytes
}
func (b *Bitmap) RGBAt(x, y int) RGB {
	if !b.boundsOK(x, y) {
		return RGB{}
	}
	return b.rgbAtUnchecked(x, y)
}

func (b *Bitmap) rgbAtUnchecked(x, y int) RGB {
	i := (y*b.Width + x) * 4
	return RGB{b.Pixels[i], b.Pixels[i+1], b.Pixels[i+2]}
}
func (b *Bitmap) ColorModel() color.Model { return color.RGBAModel }
func (b *Bitmap) Bounds() image.Rectangle {
	if b == nil || b.Width <= 0 || b.Height <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(0, 0, b.Width, b.Height)
}
func (b *Bitmap) At(x, y int) color.Color {
	if !b.boundsOK(x, y) {
		return color.RGBA{}
	}
	return b.rgbAtUnchecked(x, y).RGBA()
}
func (b *Bitmap) Set(x, y int, c RGB) {
	if !b.boundsOK(x, y) {
		return
	}
	pixel := y*b.Width + x
	if pixel > maxInt/4 {
		return
	}
	i := pixel * 4
	if i+3 >= len(b.Pixels) {
		return
	}
	b.Pixels[i], b.Pixels[i+1], b.Pixels[i+2], b.Pixels[i+3] = c.R, c.G, c.B, 255
}
func (b *Bitmap) Clone() *Bitmap {
	if !b.valid() {
		return nil
	}
	bytes, _ := bitmapByteLen(b.Width, b.Height)
	p := append([]byte(nil), b.Pixels[:bytes]...)
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
	outWidth, outHeight := r.Width(), r.Height()
	out, _ := NewBitmap(outWidth, outHeight)
	rowBytes := outWidth * 4
	sourceOffset := (r.Top*b.Width + r.Left) * 4
	for y := 0; y < outHeight; y++ {
		copy(out.Pixels[y*rowBytes:(y+1)*rowBytes], b.Pixels[sourceOffset:sourceOffset+rowBytes])
		sourceOffset += b.Width * 4
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
		if !b.boundsOK(e.Point.X, e.Point.Y) || !b.rgbAtUnchecked(e.Point.X, e.Point.Y).Matches(e.Color, e.Tolerance) {
			return false
		}
	}
	return true
}
func (b *Bitmap) WritePNG(w io.Writer) error {
	if b == nil || !b.valid() || w == nil {
		return ErrInvalidArgument
	}
	pixels := b.Pixels[:b.Width*b.Height*4]
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] == 255 {
			continue
		}
		pixels = append([]byte(nil), pixels...)
		for alpha := 3; alpha < len(pixels); alpha += 4 {
			pixels[alpha] = 255
		}
		break
	}
	rgba := &image.RGBA{
		Pix:    pixels,
		Stride: b.Width * 4,
		Rect:   image.Rect(0, 0, b.Width, b.Height),
	}
	return png.Encode(w, rgba)
}
func (b *Bitmap) SavePNG(path string) (err error) {
	if b == nil || !b.valid() {
		return ErrInvalidArgument
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	return b.WritePNG(f)
}
