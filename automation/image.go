package automation

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strconv"
	"strings"

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
	bitmap  templateBitmap
	options ImageSearchOptions
}

func CompileTemplate(img image.Image, o ImageSearchOptions) (*Template, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}
	if img.Bounds().Empty() || o.Width < -1 || o.Height < -1 {
		return nil, ErrInvalidRect
	}
	return &Template{bitmap: imageToTemplate(img, o), options: o}, nil
}
func LoadTemplate(path string, o ImageSearchOptions) (*Template, error) {
	img, e := LoadImage(path)
	if e != nil {
		return nil, e
	}
	return CompileTemplate(img, o)
}

// ParseImageSearchOptions accepts AHK-style options such as "*42 *TransBlack
// *w100 *h-1". The returned filename is the remaining, unparsed text.
func ParseImageSearchOptions(spec string) (ImageSearchOptions, string, error) {
	var o ImageSearchOptions
	fields := strings.Fields(spec)
	cut := 0
	for cut < len(fields) && strings.HasPrefix(strings.ToLower(fields[cut]), "*") {
		t := fields[cut]
		low := strings.ToLower(t)
		if n, err := strconv.Atoi(t[1:]); err == nil {
			if n < 0 || n > 255 {
				return o, "", fmt.Errorf("variation out of range: %d", n)
			}
			o.Variation = uint8(n)
			cut++
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
		cut++
	}
	return o, strings.Join(fields[cut:], " "), nil
}

func parseHexColor(v string) (RGB, error) {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "0x"), "0X")
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
	default:
		return RGB{}, false
	}
}

// LoadImage decodes PNG, JPEG, GIF and other formats registered with image.
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// SearchImageFile finds the first top-left match in row-major order.
func SearchImageFile(source *Bitmap, region Rect, path string, options ImageSearchOptions) (Point, bool, error) {
	img, err := LoadImage(path)
	if err != nil {
		return Point{}, false, err
	}
	return SearchImage(source, region, img, options)
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
	t, o := template.bitmap, template.options
	if t.Width <= 0 || t.Height <= 0 || t.Width > source.Width || t.Height > source.Height {
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
	maxX, maxY := region.Right-t.Width, region.Bottom-t.Height
	for y := region.Top; y <= maxY; y++ {
		for x := region.Left; x <= maxX; x++ {
			if imageAt(source, x, y, t, o) {
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

func imageToTemplate(src image.Image, o ImageSearchOptions) templateBitmap {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if o.Width != 0 || o.Height != 0 {
		w, h = scaledSize(w, h, o.Width, o.Height)
	}
	t := templateBitmap{Width: w, Height: h, Pixels: make([]byte, w*h*3), Alpha: make([]uint8, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			if w != b.Dx() {
				sx = x * b.Dx() / w
			}
			if h != b.Dy() {
				sy = y * b.Dy() / h
			}
			r, g, bl, a := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
			i := (y*w + x) * 3
			t.Pixels[i], t.Pixels[i+1], t.Pixels[i+2], t.Alpha[y*w+x] = uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)
		}
	}
	var eligible []int
	for i := 0; i < w*h; i++ {
		if t.Alpha[i] == 0 {
			continue
		}
		j := i * 3
		c := RGB{t.Pixels[j], t.Pixels[j+1], t.Pixels[j+2]}
		if o.Transparent == nil || c != *o.Transparent {
			eligible = append(eligible, i)
		}
	}
	if len(eligible) > 0 {
		for _, q := range []int{0, len(eligible) / 3, 2 * len(eligible) / 3, len(eligible) - 1} {
			a := eligible[q]
			duplicate := false
			for _, v := range t.Anchors {
				if v == a {
					duplicate = true
				}
			}
			if !duplicate {
				t.Anchors = append(t.Anchors, a)
			}
		}
	}
	return t
}
func scaledSize(w, h, ow, oh int) (int, int) {
	if ow == 0 && oh == 0 {
		return w, h
	}
	if ow == -1 && oh == 0 {
		ow = w
	}
	if oh == -1 && ow == 0 {
		oh = h
	}
	if ow == -1 {
		ow = max(1, w*oh/h)
	}
	if oh == -1 {
		oh = max(1, h*ow/w)
	}
	if ow <= 0 {
		ow = w
	}
	if oh <= 0 {
		oh = h
	}
	return ow, oh
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func imageAt(s *Bitmap, x, y int, t templateBitmap, o ImageSearchOptions) bool {
	for _, ti := range t.Anchors {
		tx, ty := ti%t.Width, ti/t.Width
		if !templatePixelMatches(s.RGBAt(x+tx, y+ty), t, ti, o) {
			return false
		}
	}
	for ty := 0; ty < t.Height; ty++ {
		for tx := 0; tx < t.Width; tx++ {
			ti := (ty*t.Width + tx)
			if t.Alpha[ti] == 0 {
				continue
			}
			if !templatePixelMatches(s.RGBAt(x+tx, y+ty), t, ti, o) {
				return false
			}
		}
	}
	return true
}

func templatePixelMatches(c RGB, t templateBitmap, ti int, o ImageSearchOptions) bool {
	if t.Alpha[ti] == 0 {
		return true
	}
	i := ti * 3
	tc := RGB{t.Pixels[i], t.Pixels[i+1], t.Pixels[i+2]}
	if o.Transparent != nil && tc == *o.Transparent {
		return true
	}
	return c.Matches(tc, Tolerance(o.Variation))
}

// Register the standard decoders explicitly for callers that use a custom
// image registry elsewhere in the process.
var _ = gif.GIF{}
var _ = jpeg.DefaultQuality
var _ = png.Encoder{}
