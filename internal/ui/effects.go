package ui

import (
	"image/color"
	"math"
	"math/rand"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// ---------------------------------------------------------------- text helpers

// heading returns a title in the heading face (Cinzel Decorative).
func heading(text string, size float32) *canvas.Text {
	t := canvas.NewText(text, colGold)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Italic: true} // mapped to the heading font by duneTheme
	return t
}

// caption returns small spaced-out body text.
func caption(text string, c color.Color, size float32) *canvas.Text {
	t := canvas.NewText(text, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// panel wraps content in a translucent sandstone card with a spice-coloured edge.
func panel(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel)
	bg.StrokeColor = colLine
	bg.StrokeWidth = 1
	bg.CornerRadius = 8
	return container.NewStack(bg, container.NewPadded(content))
}

// ---------------------------------------------------------------- sand motes

type mote struct {
	x, y, vx, vy, phase, wobble float32
	c                           *canvas.Circle
	base                        uint8
}

// motes is a layer of drifting sand particles. speed 1 is a calm breeze; the
// install page raises it for a "sandstorm" while work is in progress.
type motes struct {
	box   *fyne.Container
	items []*mote
	size  fyne.Size
	speed float32
	last  time.Time
	anim  *fyne.Animation
}

type moteLayout struct{ m *motes }

func (l moteLayout) Layout(_ []fyne.CanvasObject, s fyne.Size) { l.m.size = s }
func (moteLayout) MinSize(_ []fyne.CanvasObject) fyne.Size     { return fyne.NewSize(1, 1) }

func newMotes(n int) *motes {
	m := &motes{speed: 1, size: fyne.NewSize(940, 760)}
	rng := rand.New(rand.NewSource(7))
	var objs []fyne.CanvasObject
	for i := 0; i < n; i++ {
		r := 1 + rng.Float32()*2.2
		c := canvas.NewCircle(color.NRGBA{R: 0xf3, G: 0xc0, B: 0x70, A: 60})
		c.Resize(fyne.NewSize(r*2, r*2))
		it := &mote{
			x: rng.Float32() * 940, y: rng.Float32() * 760,
			vx: 8 + r*7, vy: -2 - rng.Float32()*4, phase: rng.Float32() * 6.28, wobble: 6 + rng.Float32()*14,
			c: c, base: uint8(35 + rng.Intn(90)),
		}
		m.items = append(m.items, it)
		objs = append(objs, c)
	}
	m.box = container.New(moteLayout{m}, objs...)
	return m
}

func (m *motes) start() {
	m.last = time.Now()
	m.anim = fyne.NewAnimation(time.Second, func(float32) { m.step() })
	m.anim.RepeatCount = fyne.AnimationRepeatForever
	m.anim.Curve = fyne.AnimationLinear
	m.anim.Start()
}

// setOn shows and animates the sand, or hides it and stops all work.
func (m *motes) setOn(on bool) {
	if on {
		m.box.Show()
		m.last = time.Now()
		if m.anim != nil {
			m.anim.Start()
		}
		return
	}
	if m.anim != nil {
		m.anim.Stop()
	}
	m.box.Hide()
}

func (m *motes) step() {
	now := time.Now()
	dt := float32(now.Sub(m.last).Seconds())
	m.last = now
	if dt <= 0 || dt > 0.25 {
		return
	}
	w, h := m.size.Width, m.size.Height
	for _, it := range m.items {
		it.phase += dt * (0.6 + m.speed*0.4)
		it.x += it.vx * m.speed * dt
		it.y += it.vy*m.speed*dt + float32(math.Sin(float64(it.phase)))*it.wobble*dt
		if it.x > w+6 {
			it.x = -6
		}
		if it.y < -6 {
			it.y = h + 6
		}
		if it.y > h+6 {
			it.y = -6
		}
		tw := 0.65 + 0.35*float32(math.Sin(float64(it.phase*2.3)))
		a := float32(it.base) * tw * (0.8 + 0.25*m.speed)
		if a > 220 {
			a = 220
		}
		it.c.FillColor = color.NRGBA{R: 0xf3, G: 0xc0, B: 0x70, A: uint8(a)}
		it.c.Move(fyne.NewPos(it.x, it.y))
		it.c.Refresh()
	}
}

// setStorm switches between calm drifting (false) and a fast sandstorm (true).
func (m *motes) setStorm(on bool) {
	if on {
		m.speed = 5
	} else {
		m.speed = 1
	}
}

// ---------------------------------------------------------------- shimmer line

type shimmerLayout struct {
	glow *fyne.Container
	t    float32
}

func (l *shimmerLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	base, glow := objs[0], objs[1]
	base.Move(fyne.NewPos(0, s.Height/2-1))
	base.Resize(fyne.NewSize(s.Width, 2))
	gw := float32(220)
	glow.Resize(fyne.NewSize(gw, 4))
	glow.Move(fyne.NewPos(-gw+(s.Width+gw)*l.t, s.Height/2-2))
}
func (l *shimmerLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(10, 6) }

// shimmer is a thin line with a bright glint that travels along it.
type shimmer struct {
	obj  fyne.CanvasObject
	anim *fyne.Animation
	l    *shimmerLayout
}

func newShimmer() *shimmer {
	base := canvas.NewRectangle(colLine)
	left := canvas.NewHorizontalGradient(color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0}, color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0xf0})
	right := canvas.NewHorizontalGradient(color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0xf0}, color.NRGBA{R: 0xf3, G: 0xb9, B: 0x4d, A: 0})
	glow := container.NewGridWithColumns(2, left, right)
	l := &shimmerLayout{}
	c := container.New(l, base, glow)
	sh := &shimmer{obj: c, l: l}
	sh.anim = fyne.NewAnimation(4200*time.Millisecond, func(p float32) { l.t = p; c.Refresh() })
	sh.anim.RepeatCount = fyne.AnimationRepeatForever
	sh.anim.Curve = fyne.AnimationEaseInOut
	sh.anim.Start()
	return sh
}

func (sh *shimmer) setOn(on bool) {
	if on {
		sh.anim.Start()
		return
	}
	sh.anim.Stop()
	sh.l.t = -1 // glint parked off-screen
	sh.obj.Refresh()
}

// pulser slowly breathes a text colour between two tones (spice glow).
type pulser struct {
	anim *fyne.Animation
	t    *canvas.Text
	rest color.Color
}

func newPulse(t *canvas.Text, from, to color.Color) *pulser {
	p := &pulser{t: t, rest: to}
	a := canvas.NewColorRGBAAnimation(from, to, 2400*time.Millisecond, func(c color.Color) {
		t.Color = c
		t.Refresh()
	})
	a.AutoReverse = true
	a.RepeatCount = fyne.AnimationRepeatForever
	a.Curve = fyne.AnimationEaseInOut
	a.Start()
	p.anim = a
	return p
}

func (p *pulser) setOn(on bool) {
	if on {
		p.anim.Start()
		return
	}
	p.anim.Stop()
	p.t.Color = p.rest
	p.t.Refresh()
}

// ---------------------------------------------------------------- transitions

// flasher draws a dark veil that fades away: used when a page changes.
type flasher struct {
	rect *canvas.Rectangle
	on   bool
}

func newFlasher() *flasher {
	return &flasher{rect: canvas.NewRectangle(color.NRGBA{A: 0}), on: true}
}

func (f *flasher) flash() {
	if !f.on {
		return
	}
	a := canvas.NewColorRGBAAnimation(color.NRGBA{R: 0x1a, G: 0x0c, B: 0x04, A: 215}, color.NRGBA{R: 0x1a, G: 0x0c, B: 0x04, A: 0}, 420*time.Millisecond, func(c color.Color) {
		f.rect.FillColor = c
		f.rect.Refresh()
	})
	a.Curve = fyne.AnimationEaseOut
	a.Start()
}

// ---------------------------------------------------------------- step bar

// stepBar shows the wizard progress as a row of spaced captions.
type stepBar struct {
	box   *fyne.Container
	steps []*canvas.Text
	seps  []*canvas.Text
}

func newStepBar(names []string) *stepBar {
	s := &stepBar{}
	var objs []fyne.CanvasObject
	for i, n := range names {
		t := caption(spaced(n), colSandDim, 14)
		s.steps = append(s.steps, t)
		objs = append(objs, t)
		if i < len(names)-1 {
			sep := caption("◆", colLine, 9)
			s.seps = append(s.seps, sep)
			objs = append(objs, sep)
		}
	}
	s.box = container.NewHBox(objs...)
	return s
}

func (s *stepBar) set(cur int) {
	for i, t := range s.steps {
		switch {
		case i == cur:
			t.Color = colGold
		case i < cur:
			t.Color = colSpice
		default:
			t.Color = colSandDim
		}
		t.Refresh()
	}
}

// spaced upper-cases text and adds hair spaces between letters for a
// widescreen sci-fi look.
func spaced(s string) string {
	out := make([]rune, 0, len(s)*2)
	for i, r := range []rune(s) {
		if i > 0 {
			out = append(out, ' ')
		}
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		out = append(out, r)
	}
	return string(out)
}
