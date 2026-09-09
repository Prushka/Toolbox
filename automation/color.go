package automation

// HSV is a hue/saturation/value color. Hue is in degrees [0, 360); saturation
// and value are in [0, 1].
type HSV struct{ H, S, V float64 }

// HSV converts the color to hue/saturation/value space.
func (c RGB) HSV() HSV {
	// Compare exact byte channels first. Hue uses channel ratios, so there is
	// no need to normalize three channels or apply floating-point modulus.
	maxC, minC := max(c.R, c.G, c.B), min(c.R, c.G, c.B)
	delta := float64(maxC - minC)
	hsv := HSV{V: float64(maxC) / 255}
	if maxC > 0 {
		hsv.S = delta / float64(maxC)
	}
	if delta == 0 {
		return hsv
	}
	var h float64
	switch maxC {
	case c.R:
		h = float64(int(c.G)-int(c.B)) / delta
	case c.G:
		h = float64(int(c.B)-int(c.R))/delta + 2
	default:
		h = float64(int(c.R)-int(c.G))/delta + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	hsv.H = h
	return hsv
}

// ColorRange classifies colors by hue window and saturation/value bounds.
// Hue-based classification survives display tone mapping (for example an SDR
// application composed onto an HDR desktop) better than a fixed RGB box,
// because tone mapping shifts brightness and saturation far more than hue.
//
// Hue bounds wrap around zero when HueMin > HueMax, so a red class can be
// expressed as HueMin 340, HueMax 20. A zero SatMax or ValMax means 1.
type ColorRange struct {
	HueMin, HueMax float64
	SatMin, SatMax float64
	ValMin, ValMax float64
}

// Matches reports whether the color falls inside the range. Colors with very
// low saturation have an undefined hue; they match only when SatMin is zero.
func (r ColorRange) Matches(c RGB) bool {
	return r.MatchesHSV(c.HSV())
}

// MatchesHSV is Matches for an already converted color.
func (r ColorRange) MatchesHSV(hsv HSV) bool {
	satMax, valMax := r.SatMax, r.ValMax
	if satMax == 0 {
		satMax = 1
	}
	if valMax == 0 {
		valMax = 1
	}
	if !(hsv.S >= r.SatMin && hsv.S <= satMax && hsv.V >= r.ValMin && hsv.V <= valMax) {
		return false
	}
	if r.HueMin == 0 && r.HueMax == 0 {
		return true
	}
	if r.HueMin <= r.HueMax {
		return hsv.H >= r.HueMin && hsv.H <= r.HueMax
	}
	return hsv.H >= r.HueMin || hsv.H <= r.HueMax
}

// Luma returns the Rec. 601 luminance of the color in [0, 255].
func (c RGB) Luma() uint8 {
	return uint8((299*int(c.R) + 587*int(c.G) + 114*int(c.B) + 500) / 1000)
}
