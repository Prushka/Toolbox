package automation

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// ImageSearchOptions mirrors the useful AHK ImageSearch options. Variation is
// the allowed per-channel difference (0..255). Transparent makes matching
// pixels of that exact template color wildcards. Width and Height optionally
// scale the template; -1 preserves aspect ratio.
type ImageSearchOptions struct {
	Variation     uint8
	Transparent   *RGB
	Width, Height int
}

// Template is a predecoded, prescaled image optimized for repeated searches.
type Template struct {
	bitmap          templateBitmap
	options         ImageSearchOptions
	correlation     templateCorrelation
	correlationOnce sync.Once
}

// Size returns the compiled template dimensions.
func (t *Template) Size() (width, height int) {
	if t == nil {
		return 0, 0
	}
	return t.bitmap.Width, t.bitmap.Height
}

const maxTemplateBytes = 256 << 20

func CompileTemplate(img image.Image, o ImageSearchOptions) (*Template, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}
	if img.Bounds().Empty() || o.Width < -1 || o.Height < -1 {
		return nil, ErrInvalidRect
	}
	o = cloneImageSearchOptions(o)
	b, err := imageToTemplate(img, o)
	if err != nil {
		return nil, err
	}
	// Transparency is compiled into the alpha mask, keeping the hot search
	// loop pointer-free and independent of caller-owned option storage.
	o.Transparent = nil
	return &Template{bitmap: b, options: o}, nil
}

func cloneImageSearchOptions(o ImageSearchOptions) ImageSearchOptions {
	if o.Transparent != nil {
		c := *o.Transparent
		o.Transparent = &c
	}
	return o
}

func LoadTemplate(path string, o ImageSearchOptions) (*Template, error) {
	img, err := loadImage(path)
	if err != nil {
		return nil, err
	}
	return CompileTemplate(img, o)
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	bytes, ok := bitmapByteLen(config.Width, config.Height)
	if !ok || bytes > maxTemplateBytes {
		return nil, ErrInvalidRect
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	bytes, ok = bitmapByteLen(bounds.Dx(), bounds.Dy())
	if !ok || bytes > maxTemplateBytes {
		return nil, ErrInvalidRect
	}
	return img, nil
}

// ParseImageSearchOptions accepts AHK-style options such as "*42 *TransBlack
// *w100 *h-1". The returned filename is the remaining, unparsed text.
func ParseImageSearchOptions(spec string) (ImageSearchOptions, string, error) {
	var o ImageSearchOptions
	rest := strings.TrimLeftFunc(spec, unicode.IsSpace)
	for strings.HasPrefix(rest, "*") {
		end := strings.IndexFunc(rest, unicode.IsSpace)
		if end < 0 {
			end = len(rest)
		}
		t := rest[:end]
		low := strings.ToLower(t)
		if n, err := strconv.Atoi(t[1:]); err == nil {
			if n < 0 || n > 255 {
				return o, "", fmt.Errorf("variation out of range: %d", n)
			}
			o.Variation = uint8(n)
			rest = strings.TrimLeftFunc(rest[end:], unicode.IsSpace)
			continue
		}
		switch {
		case strings.HasPrefix(low, "*trans"):
			v := t[6:]
			if c, ok := namedColor(v); ok {
				o.Transparent = &c
			} else if c, err := parseHexColor(v); err == nil {
				o.Transparent = &c
			} else {
				return o, "", fmt.Errorf("invalid transparency color %q", v)
			}
		case strings.HasPrefix(low, "*w"):
			n, err := strconv.Atoi(t[2:])
			if err != nil || n < -1 {
				return o, "", fmt.Errorf("invalid image width %q", t)
			}
			o.Width = n
		case strings.HasPrefix(low, "*h"):
			n, err := strconv.Atoi(t[2:])
			if err != nil || n < -1 {
				return o, "", fmt.Errorf("invalid image height %q", t)
			}
			o.Height = n
		default:
			return o, "", fmt.Errorf("unsupported ImageSearch option %q", t)
		}
		rest = strings.TrimLeftFunc(rest[end:], unicode.IsSpace)
	}
	path := strings.TrimSpace(rest)
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		path = path[1 : len(path)-1]
	}
	return o, path, nil
}

func parseHexColor(v string) (RGB, error) {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "0x"), "0X")
	v = strings.TrimPrefix(v, "#")
	if len(v) != 6 {
		return RGB{}, fmt.Errorf("expected RRGGBB")
	}
	n, err := strconv.ParseUint(v, 16, 24)
	if err != nil {
		return RGB{}, err
	}
	return RGBFromUint32(uint32(n)), nil
}
func namedColor(v string) (RGB, bool) {
	switch strings.ToLower(v) {
	case "black":
		return RGB{0, 0, 0}, true
	case "white":
		return RGB{255, 255, 255}, true
	case "red":
		return RGB{255, 0, 0}, true
	case "green":
		return RGB{0, 128, 0}, true
	case "blue":
		return RGB{0, 0, 255}, true
	case "yellow":
		return RGB{255, 255, 0}, true
	case "magenta", "fuchsia":
		return RGB{255, 0, 255}, true
	case "cyan", "aqua":
		return RGB{0, 255, 255}, true
	case "gray", "grey":
		return RGB{128, 128, 128}, true
	case "silver":
		return RGB{192, 192, 192}, true
	case "maroon":
		return RGB{128, 0, 0}, true
	case "purple":
		return RGB{128, 0, 128}, true
	case "lime":
		return RGB{0, 255, 0}, true
	case "olive":
		return RGB{128, 128, 0}, true
	case "navy":
		return RGB{0, 0, 128}, true
	case "teal":
		return RGB{0, 128, 128}, true
	case "orange":
		return RGB{255, 165, 0}, true
	default:
		return RGB{}, false
	}
}

// LoadImage decodes PNG, JPEG, GIF and other formats registered with image.
func LoadImage(path string) (image.Image, error) {
	return loadImage(path)
}

// SearchImageFile finds the first top-left match in row-major order.
func SearchImageFile(source *Bitmap, region Rect, path string, options ImageSearchOptions) (Point, bool, error) {
	template, err := LoadTemplate(path, options)
	if err != nil {
		return Point{}, false, err
	}
	return SearchTemplate(source, region, template)
}

// SearchImageFileSpec accepts one AHK-style ImageFile string containing both
// options and the path (for example "*42 *TransBlack icon.png").
func SearchImageFileSpec(source *Bitmap, region Rect, spec string) (Point, bool, error) {
	o, path, e := ParseImageSearchOptions(spec)
	if e != nil {
		return Point{}, false, e
	}
	if path == "" {
		return Point{}, false, ErrInvalidArgument
	}
	return SearchImageFile(source, region, path, o)
}

// SearchImage implements the matching semantics used by AHK ImageSearch.
// The search region is clipped to source bounds; a zero region means all of
// source. Alpha-zero template pixels are always wildcards.
func SearchImage(source *Bitmap, region Rect, template image.Image, options ImageSearchOptions) (Point, bool, error) {
	t, err := CompileTemplate(template, options)
	if err != nil {
		return Point{}, false, err
	}
	return SearchTemplate(source, region, t)
}

func SearchTemplate(source *Bitmap, region Rect, template *Template) (Point, bool, error) {
	if source == nil || template == nil {
		return Point{}, false, fmt.Errorf("nil image")
	}
	if !source.valid() {
		return Point{}, false, ErrInvalidArgument
	}
	t, o := template.bitmap, template.options
	if !t.valid() {
		return Point{}, false, ErrInvalidArgument
	}
	if t.Width > source.Width || t.Height > source.Height {
		return Point{}, false, nil
	}
	if region == (Rect{}) {
		region = Rect{0, 0, source.Width, source.Height}
	}
	region = region.Normalize()
	if region.Empty() {
		return Point{}, false, nil
	}
	if region.Left < 0 {
		region.Left = 0
	}
	if region.Top < 0 {
		region.Top = 0
	}
	if region.Right > source.Width {
		region.Right = source.Width
	}
	if region.Bottom > source.Height {
		region.Bottom = source.Height
	}
	if region.Empty() {
		return Point{}, false, nil
	}
	maxX, maxY := region.Right-t.Width, region.Bottom-t.Height
	if maxX < region.Left || maxY < region.Top {
		return Point{}, false, nil
	}
	if len(t.Anchors) == 0 {
		return Point{region.Left, region.Top}, true, nil
	}
	tolerance := Tolerance(o.Variation)
	exact := tolerance == (ColorTolerance{})
	for y := region.Top; y <= maxY; y++ {
		for x := region.Left; x <= maxX; x++ {
			if imageAt(source, x, y, t, tolerance, exact) {
				return Point{x, y}, true, nil
			}
		}
	}
	return Point{}, false, nil
}

type templateBitmap struct {
	Width, Height int
	Pixels        []byte
	Alpha         []uint8
	Anchors       []int
}

func (t templateBitmap) valid() bool {
	bytes, ok := bitmapByteLen(t.Width, t.Height)
	if !ok || bytes > maxTemplateBytes {
		return false
	}
	pixels := bytes / 4
	if len(t.Pixels) < pixels*3 || len(t.Alpha) < pixels {
		return false
	}
	for _, anchor := range t.Anchors {
		if anchor < 0 || anchor >= pixels {
			return false
		}
	}
	return true
}

func imageToTemplate(src image.Image, o ImageSearchOptions) (templateBitmap, error) {
	b := src.Bounds()
	sourceWidth, sourceHeight := b.Dx(), b.Dy()
	w, h := sourceWidth, sourceHeight
	if o.Width != 0 || o.Height != 0 {
		var err error
		w, h, err = scaledSize(w, h, o.Width, o.Height)
		if err != nil {
			return templateBitmap{}, err
		}
	}
	bytes, ok := bitmapByteLen(w, h)
	if !ok || bytes > maxTemplateBytes {
		return templateBitmap{}, ErrInvalidRect
	}
	pixelCount := bytes / 4
	t := templateBitmap{Width: w, Height: h, Pixels: make([]byte, pixelCount*3), Alpha: make([]uint8, pixelCount)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			if w != sourceWidth {
				sx = scaleIndex(x, sourceWidth, w)
			}
			if h != sourceHeight {
				sy = scaleIndex(y, sourceHeight, h)
			}
			c := color.NRGBAModel.Convert(src.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA)
			i := (y*w + x) * 3
			rgb := RGB{c.R, c.G, c.B}
			alpha := c.A
			if o.Transparent != nil && rgb == *o.Transparent {
				alpha = 0
			}
			t.Pixels[i], t.Pixels[i+1], t.Pixels[i+2], t.Alpha[y*w+x] = rgb.R, rgb.G, rgb.B, alpha
		}
	}
	eligibleCount := 0
	for i := 0; i < w*h; i++ {
		if t.Alpha[i] == 0 {
			continue
		}
		eligibleCount++
	}
	if eligibleCount > 0 {
		ordinals := [4]int{0, eligibleCount / 3, 2 * eligibleCount / 3, eligibleCount - 1}
		eligibleOrdinal := 0
		ordinalSlot := 0
		for i := 0; i < w*h && ordinalSlot < len(ordinals); i++ {
			if t.Alpha[i] == 0 {
				continue
			}
			if eligibleOrdinal == ordinals[ordinalSlot] {
				t.Anchors = append(t.Anchors, i)
				selectedOrdinal := ordinals[ordinalSlot]
				for ordinalSlot < len(ordinals) && ordinals[ordinalSlot] == selectedOrdinal {
					ordinalSlot++
				}
			}
			eligibleOrdinal++
		}
	}
	return t, nil
}

// scaleIndex computes index*source/destination without overflowing. index is
// always smaller than destination and template dimensions are allocation-capped.
func scaleIndex(index, source, destination int) int {
	q, rem := source/destination, source%destination
	return index*q + int(int64(index)*int64(rem)/int64(destination))
}

func scaledSize(w, h, ow, oh int) (int, int, error) {
	if w <= 0 || h <= 0 || ow < -1 || oh < -1 {
		return 0, 0, ErrInvalidRect
	}
	if ow == 0 && oh == 0 {
		return w, h, nil
	}
	if ow == -1 && oh == 0 {
		ow = w
	}
	if oh == -1 && ow == 0 {
		oh = h
	}
	if ow == -1 {
		if oh <= 0 || w > maxInt/oh {
			return 0, 0, ErrInvalidRect
		}
		ow = max(1, w*oh/h)
	}
	if oh == -1 {
		if ow <= 0 || h > maxInt/ow {
			return 0, 0, ErrInvalidRect
		}
		oh = max(1, h*ow/w)
	}
	if ow <= 0 {
		ow = w
	}
	if oh <= 0 {
		oh = h
	}
	if _, ok := bitmapByteLen(ow, oh); !ok {
		return 0, 0, ErrInvalidRect
	}
	return ow, oh, nil
}
func imageAt(s *Bitmap, x, y int, t templateBitmap, tolerance ColorTolerance, exact bool) bool {
	for _, ti := range t.Anchors {
		tx, ty := ti%t.Width, ti/t.Width
		si := ((y+ty)*s.Width + x + tx) * 4
		if !templatePixelMatches(s.Pixels, si, t.Pixels, ti*3, tolerance, exact) {
			return false
		}
	}
	anchorSlot := 0
	ti := 0
	for ty := 0; ty < t.Height; ty++ {
		si := ((y+ty)*s.Width + x) * 4
		for tx := 0; tx < t.Width; tx++ {
			if anchorSlot < len(t.Anchors) && ti == t.Anchors[anchorSlot] {
				anchorSlot++
			} else if t.Alpha[ti] != 0 && !templatePixelMatches(s.Pixels, si, t.Pixels, ti*3, tolerance, exact) {
				return false
			}
			ti++
			si += 4
		}
	}
	return true
}

func templatePixelMatches(source []byte, sourceIndex int, template []byte, templateIndex int, tolerance ColorTolerance, exact bool) bool {
	if exact {
		return source[sourceIndex] == template[templateIndex] &&
			source[sourceIndex+1] == template[templateIndex+1] &&
			source[sourceIndex+2] == template[templateIndex+2]
	}
	return abs(source[sourceIndex], template[templateIndex]) <= tolerance.R &&
		abs(source[sourceIndex+1], template[templateIndex+1]) <= tolerance.G &&
		abs(source[sourceIndex+2], template[templateIndex+2]) <= tolerance.B
}

// Register the standard decoders explicitly for callers that use a custom
// image registry elsewhere in the process.
var _ = gif.GIF{}
var _ = jpeg.DefaultQuality
var _ = png.Encoder{}
