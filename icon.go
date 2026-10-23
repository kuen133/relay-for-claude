package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"
)

// The tray icon is a ring that fills with the active account's session usage
// and turns orange, then red, as it gets close to the limit.

var (
	colorOK      = color.RGBA{0x34, 0xC7, 0x59, 0xFF}
	colorWarn    = color.RGBA{0xFF, 0x9F, 0x0A, 0xFF}
	colorFull    = color.RGBA{0xFF, 0x3B, 0x30, 0xFF}
	colorUnknown = color.RGBA{0x8E, 0x8E, 0x93, 0xFF}
	colorTrack   = color.RGBA{0x8E, 0x8E, 0x93, 0x70}
)

const warnAt = 75

func usageColor(pct, threshold float64) color.RGBA {
	switch {
	case pct >= threshold:
		return colorFull
	case pct >= warnAt:
		return colorWarn
	}
	return colorOK
}

// trayIcon returns the icon in the format the platform tray expects. On a Mac
// it is drawn at twice the 20pt status bar size so it stays sharp.
func trayIcon(pct, threshold float64) []byte {
	if runtime.GOOS == "windows" {
		return encodeICO(drawIcon(pct, threshold))
	}
	return pngIcon(drawIconSize(40, pct, threshold))
}

func pngIcon(img *image.NRGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// drawIcon draws the ring at the 32 pixel size of the Windows tray.
func drawIcon(pct, threshold float64) *image.NRGBA { return drawIconSize(32, pct, threshold) }

// drawIconSize draws a thick ring that fills clockwise from twelve o'clock
// with round ends, plus a dot in the usage colour. pct < 0 means unknown.
func drawIconSize(size int, pct, threshold float64) *image.NRGBA {
	const samples = 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	fill := colorUnknown
	if pct >= 0 {
		fill = usageColor(pct, threshold)
	}
	frac := math.Min(math.Max(pct, 0), 100) / 100
	c := float64(size) / 2
	outer, inner, dot := 0.49*float64(size), 0.29*float64(size), 0.12*float64(size)
	mid, half := (outer+inner)/2, (outer-inner)/2
	// Centres of the round ends: the start at twelve o'clock and the tip.
	tipX, tipY := mid*math.Sin(2*math.Pi*frac), -mid*math.Cos(2*math.Pi*frac)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := float64(x) + (float64(sx)+0.5)/samples - c
					py := float64(y) + (float64(sy)+0.5)/samples - c
					d := math.Hypot(px, py)
					angle := math.Mod(math.Atan2(px, -py)+2*math.Pi, 2*math.Pi) / (2 * math.Pi)
					filled := pct >= 0 && frac > 0 && ((d >= inner && d <= outer && angle <= frac) ||
						math.Hypot(px, py+mid) <= half || math.Hypot(px-tipX, py-tipY) <= half)
					var col color.RGBA
					switch {
					case filled:
						col = fill
					case d >= inner && d <= outer:
						col = colorTrack
					case d <= dot:
						col = fill
					default:
						continue
					}
					w := float64(col.A) / 255
					r, g, b, a = r+float64(col.R)*w, g+float64(col.G)*w, b+float64(col.B)*w, a+w
				}
			}
			if a == 0 {
				continue
			}
			n := float64(samples * samples)
			img.SetNRGBA(x, y, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(a / n * 255)})
		}
	}
	return img
}

// encodeICO wraps a 32-bit image in a single-image .ico, the format the
// Windows tray expects.
func encodeICO(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	maskRow := (w + 31) / 32 * 4
	pixels := w * h * 4
	var buf bytes.Buffer
	le := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	le([3]uint16{0, 1, 1})
	le([4]uint8{uint8(w), uint8(h), 0, 0})
	le([2]uint16{1, 32})
	le([2]uint32{uint32(40 + pixels + maskRow*h), 22})
	le([3]uint32{40, uint32(w), uint32(h * 2)})
	le([2]uint16{1, 32})
	le([6]uint32{0, uint32(pixels + maskRow*h), 0, 0, 0, 0})
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			p := img.NRGBAAt(x, y)
			buf.Write([]byte{p.B, p.G, p.R, p.A})
		}
	}
	buf.Write(make([]byte, maskRow*h))
	return buf.Bytes()
}
