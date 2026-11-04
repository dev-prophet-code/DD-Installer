package ui

import (
	"image"
	"image/color"
	"math"
	"math/rand"
)

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(lerp(float64(a.R), float64(b.R), t)),
		G: uint8(lerp(float64(a.G), float64(b.G), t)),
		B: uint8(lerp(float64(a.B), float64(b.B), t)),
		A: 255,
	}
}

// renderBackdrop paints an original Arrakis night scene: a gradient sky with
// stars, two moons and layered dune ridges. Deterministic (seeded), so it is
// identical on every start and on every operating system.
func renderBackdrop(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(20241104))

	top := color.NRGBA{R: 0x07, G: 0x05, B: 0x0e, A: 255}
	mid := color.NRGBA{R: 0x3a, G: 0x1a, B: 0x14, A: 255}
	low := color.NRGBA{R: 0xd2, G: 0x68, B: 0x1c, A: 255}
	horizon := float64(h) * 0.58
	for y := 0; y < h; y++ {
		fy := float64(y) / horizon
		var c color.NRGBA
		switch {
		case fy < 0.6:
			c = mix(top, mid, fy/0.6)
		case fy <= 1.0:
			c = mix(mid, low, math.Pow((fy-0.6)/0.4, 1.6))
		default:
			c = low
		}
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}

	// stars
	for i := 0; i < 220; i++ {
		x, y := rng.Intn(w), rng.Intn(int(horizon*0.8))
		b := uint8(120 + rng.Intn(130))
		blend(img, x, y, color.NRGBA{R: b, G: b, B: uint8(float64(b) * 0.9), A: 255}, 0.35+rng.Float64()*0.5)
	}

	// two moons with soft glow
	moon(img, int(float64(w)*0.74), int(float64(h)*0.2), 34, color.NRGBA{R: 0xf1, G: 0xdc, B: 0xb0, A: 255})
	moon(img, int(float64(w)*0.62), int(float64(h)*0.13), 16, color.NRGBA{R: 0xd7, G: 0xe0, B: 0xea, A: 255})

	// dune ridges, far to near
	layers := []struct {
		base, amp float64
		col       color.NRGBA
		seed      float64
	}{
		{0.58, 0.030, color.NRGBA{R: 0x9a, G: 0x42, B: 0x16, A: 255}, 1.3},
		{0.66, 0.045, color.NRGBA{R: 0x70, G: 0x2e, B: 0x12, A: 255}, 3.1},
		{0.76, 0.060, color.NRGBA{R: 0x48, G: 0x1e, B: 0x0e, A: 255}, 5.7},
		{0.88, 0.075, color.NRGBA{R: 0x24, G: 0x0f, B: 0x07, A: 255}, 8.2},
	}
	for _, l := range layers {
		for x := 0; x < w; x++ {
			fx := float64(x) / float64(w)
			y := (l.base + l.amp*(math.Sin(fx*5.2+l.seed)*0.6+math.Sin(fx*11.3+l.seed*1.7)*0.25+math.Sin(fx*2.1+l.seed*0.4)*0.9)) * float64(h)
			for yy := int(y); yy < h; yy++ {
				// light rim on the ridge crest
				col := l.col
				if yy-int(y) < 2 {
					col = mix(l.col, color.NRGBA{R: 0xe0, G: 0x7b, B: 0x1a, A: 255}, 0.28)
				}
				img.SetNRGBA(x, yy, col)
			}
		}
	}

	// film grain + vignette
	cx, cy := float64(w)/2, float64(h)/2
	maxd := math.Hypot(cx, cy)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			v := 1 - 0.38*math.Pow(math.Hypot(float64(x)-cx, float64(y)-cy)/maxd, 2.2)
			n := 1 + (rng.Float64()-0.5)*0.05
			img.SetNRGBA(x, y, color.NRGBA{R: clamp8(float64(c.R) * v * n), G: clamp8(float64(c.G) * v * n), B: clamp8(float64(c.B) * v * n), A: 255})
		}
	}
	return img
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func blend(img *image.NRGBA, x, y int, c color.NRGBA, a float64) {
	if !(image.Pt(x, y).In(img.Rect)) {
		return
	}
	o := img.NRGBAAt(x, y)
	img.SetNRGBA(x, y, mix(o, c, a))
}

func moon(img *image.NRGBA, cx, cy, r int, col color.NRGBA) {
	for dy := -r * 4; dy <= r*4; dy++ {
		for dx := -r * 4; dx <= r*4; dx++ {
			d := math.Hypot(float64(dx), float64(dy))
			switch {
			case d <= float64(r):
				shade := 0.82 + 0.18*(1-d/float64(r))
				blend(img, cx+dx, cy+dy, color.NRGBA{R: clamp8(float64(col.R) * shade), G: clamp8(float64(col.G) * shade), B: clamp8(float64(col.B) * shade), A: 255}, 1)
			case d <= float64(r*4):
				a := math.Pow(1-(d-float64(r))/float64(r*3), 3) * 0.28
				blend(img, cx+dx, cy+dy, col, a)
			}
		}
	}
}
