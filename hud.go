package main

import (
	"embed"
	"io/fs"
	"math"
	"strconv"

	"github.com/nsgo-dev/nsgo/core"
	evstrings "me.oneo/embervigil/strings"
)

// embeddedHUD holds the HUD kit; the whole folder is embedded so the game still builds, and draws its procedural HUD, without the PNGs.
//
//go:embed assets/hud
var embeddedHUD embed.FS

// The HUD kit's pieces, which assets/hud/README.md sizes at twice the points they show at on a 1280×720 window.
const (
	hudHealthFrame = "health_frame"
	hudHealthFill  = "health_fill"
	hudXPFrame     = "xp_frame"
	hudXPFill      = "xp_fill"
	hudBossFrame   = "boss_frame"
	hudBossFill    = "boss_fill"
	hudTimer       = "timer_plaque"
	hudLevelBadge  = "level_badge"
	hudKillEmblem  = "kill_emblem"
	hudSlotEmpty   = "slot_empty"
	hudSlotFilled  = "slot_filled"
	hudSlotMaxed   = "slot_maxed"
	hudPipLit      = "pip_lit"
	hudPipUnlit    = "pip_unlit"
	hudPause       = "pause_normal"
	hudPausePushed = "pause_pressed"
	// The touch stick's pieces, which the shell draws under the HUD.
	hudStickBase       = "stick_base"
	hudStickKnob       = "stick_knob_idle"
	hudStickKnobActive = "stick_knob_active"
)

// The fills' active ranges in the kit's pixels, from assets/hud/README.md: health empties from the top of its glass chamber, XP and the boss bar fill from the left of their troughs.
const (
	healthFillTop, healthFillBottom = 77, 155
	xpFillLeft, xpFillRight         = 20, 400
	bossFillLeft, bossFillRight     = 84, 593
)

// The HUD's layout in points at a 1280×720 window, which every piece scales with.
const (
	hudRefWidth, hudRefHeight = 1280, 720
	hudMinScale, hudMaxScale  = 0.5, 2
	hudMargin                 = 12
	hudGapPt                  = 8
	healthW, healthH          = 56, 104
	xpW, xpH                  = 210, 24
	badgeSide                 = 48
	emblemSide                = 40
	slotSide                  = 48
	slotGap                   = 6
	slotIconSide              = 32
	pipSide                   = 12
	timerW, timerH            = 120, 52
	bossW, bossH              = 340, 84
	pauseSide                 = 52
	digitW, digitH            = 24, 32
	colonW                    = 14
)

// How the HUD moves: how fast fills ease toward their values, and how long a hit flash, a level-up glow and a new weapon's pop last, in seconds.
const (
	hudEaseRate  = 10
	hudFlashTime = 0.35
	hudGlowTime  = 1.2
	hudPopTime   = 0.45
)

// HUD colours, for the canvas text and the procedural pieces drawn when the kit is missing.
var (
	hudIvory   = core.RGB(0xf6e7c1)
	hudOutline = core.RGBA(0x120b06, 0.9)
	hudIron    = core.RGBA(0x1b1712, 0.88)
	hudBronze  = core.RGB(0x9a6a3a)
	hudGold    = core.RGB(0xe0b25a)
	hudOil     = core.RGB(0xe0662a)
	hudBossRed = core.RGB(0xb3261e)
)

// hudArt is the decoded HUD kit; a missing piece is drawn procedurally.
type hudArt struct {
	images map[string]core.ImageSource
	digits [10]core.ImageSource
	colon  core.ImageSource
	times  core.ImageSource
	// numerals is whether every digit, the colon and the times sign loaded, so a number never mixes painted and system glyphs.
	numerals bool
}

// newHUDArt decodes every PNG under assets/hud/ in fsys, skipping what is missing or unreadable.
func newHUDArt(fsys fs.FS) *hudArt {
	a := &hudArt{images: loadKit(fsys, "assets/hud")}
	a.numerals = true
	for i := range a.digits {
		img, ok := a.images["digit_"+strconv.Itoa(i)]
		a.digits[i] = img
		a.numerals = a.numerals && ok
	}
	var ok bool
	if a.colon, ok = a.images["digit_colon"]; !ok {
		a.numerals = false
	}
	if a.times, ok = a.images["digit_times"]; !ok {
		a.numerals = false
	}
	return a
}

// loadKit decodes every PNG in dir of fsys by its name without the extension, skipping what is missing or unreadable.
func loadKit(fsys fs.FS, dir string) map[string]core.ImageSource {
	images := map[string]core.ImageSource{}
	if fsys == nil {
		return images
	}
	entries, _ := fs.ReadDir(fsys, dir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || len(name) < 5 || name[len(name)-4:] != ".png" {
			continue
		}
		raw, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			continue
		}
		rgba, err := decodePNGToRGBA(raw)
		if err != nil {
			continue
		}
		b := rgba.Bounds()
		images[name[:len(name)-4]] = core.ImageSource{
			Pixels:  rgba,
			Version: 1,
			Size:    core.Size{Width: core.Points(b.Dx()), Height: core.Points(b.Dy())},
		}
	}
	return images
}

// image returns the named piece, or false when the kit lacks it.
func (a *hudArt) image(name string) (core.ImageSource, bool) {
	if a == nil {
		return core.ImageSource{}, false
	}
	img, ok := a.images[name]
	return img, ok
}

// rect is a rectangle in the canvas's points.
type rect struct{ x, y, w, h float32 }

func (r rect) contains(p core.Point) bool {
	return float32(p.X) >= r.x && float32(p.X) <= r.x+r.w && float32(p.Y) >= r.y && float32(p.Y) <= r.y+r.h
}

func (r rect) center() (float32, float32) { return r.x + r.w/2, r.y + r.h/2 }

// grow returns r widened about its centre to at least side points each way.
func (r rect) grow(side float32) rect {
	if r.w < side {
		r.x -= (side - r.w) / 2
		r.w = side
	}
	if r.h < side {
		r.y -= (side - r.h) / 2
		r.h = side
	}
	return r
}

// hudLayout places the HUD's pieces in a canvas of one size and safe area.
type hudLayout struct {
	scale                        float32
	health, xp, badge, emblem    rect
	killsX, killsY, killsH       float32
	slots                        [maxHUDWeapons]rect
	timer, boss, pause, pauseHit rect
	bottom                       float32
}

// layoutHUD lays the HUD out at the canvas's real size: health, XP, level, kills and slots top-left, the timer and the boss bar top-centre and pause top-right, all inside the safe area.
func layoutHUD(size core.Size, safe core.Edges) hudLayout {
	w, h := float32(size.Width), float32(size.Height)
	s := min(w/hudRefWidth, h/hudRefHeight)
	s = max(hudMinScale, min(hudMaxScale, s))
	left := float32(safe.Left) + hudMargin
	top := float32(safe.Top) + hudMargin
	right := w - float32(safe.Right) - hudMargin

	l := hudLayout{scale: s}
	l.health = rect{left, top, healthW * s, healthH * s}
	x0 := left + (healthW+hudGapPt)*s
	l.xp = rect{x0, top + 6*s, xpW * s, xpH * s}
	l.badge = rect{x0, top + 36*s, badgeSide * s, badgeSide * s}
	l.emblem = rect{x0 + (badgeSide+hudGapPt)*s, top + 40*s, emblemSide * s, emblemSide * s}
	l.killsX = l.emblem.x + l.emblem.w + 2*s
	l.killsH = 24 * s
	l.killsY = l.emblem.y + (l.emblem.h-l.killsH)/2
	slotY := top + 92*s
	for i := range l.slots {
		l.slots[i] = rect{x0 + float32(i)*(slotSide+slotGap)*s, slotY, slotSide * s, slotSide * s}
	}
	l.bottom = slotY + (slotSide+pipSide/2)*s

	// The pause button never shrinks under a thumb's width, however small the window.
	ps := max(pauseSide*s, buttonSide)
	l.pause = rect{right - ps, top, ps, ps}
	l.pauseHit = l.pause.grow(buttonSide)

	mid := w / 2
	l.timer = rect{mid - timerW*s/2, top, timerW * s, timerH * s}
	// The boss bar shrinks before it would reach the left cluster or the pause button.
	clusterRight := x0 + xpW*s + hudGapPt
	half := min(mid-clusterRight, l.pauseHit.x-hudGapPt-mid)
	bw := min(bossW*s, 2*max(0, half))
	bh := bw * bossH / bossW
	l.boss = rect{mid - bw/2, top + (timerH+4)*s, bw, bh}
	return l
}

// hudMotion is how the HUD moves between the values it shows: eased fills, a hit flash, the level-up glow, new weapons popping in and the lantern's flicker. It runs on the shell's fixed steps and never touches the run.
type hudMotion struct {
	primed           bool
	health, xp, boss float64
	flash, glow      float64
	pops             [maxHUDWeapons]float64
	clock            float64
	last             hudState
}

// step eases the HUD toward st over dt seconds and starts the effects st's changes call for.
func (m *hudMotion) step(st hudState, dt float64) {
	health, xp, boss := st.healthFraction(), st.xpFraction(), st.bossFraction()
	if !m.primed {
		m.primed = true
		m.health, m.xp, m.boss = health, xp, boss
		m.last = st
		return
	}
	if st.health < m.last.health {
		m.flash = 1
	}
	if st.level > m.last.level {
		m.glow = 1
		m.xp = 0
	}
	if st.bossActive && (!m.last.bossActive || st.bossName != m.last.bossName) {
		m.boss = boss
	}
	for i, w := range st.weapons {
		if w.id != "" && w.id != m.last.weapons[i].id {
			m.pops[i] = 1
		}
	}
	k := 1 - math.Exp(-hudEaseRate*dt)
	m.health += (health - m.health) * k
	m.xp += (xp - m.xp) * k
	m.boss += (boss - m.boss) * k
	m.flash = max(0, m.flash-dt/hudFlashTime)
	m.glow = max(0, m.glow-dt/hudGlowTime)
	for i := range m.pops {
		m.pops[i] = max(0, m.pops[i]-dt/hudPopTime)
	}
	m.clock += dt
	m.last = st
}

// flicker is the lantern's brightness from 0 to 1 while health is low, which two out-of-step waves make uneven.
func (m *hudMotion) flicker() float64 {
	v := 0.5 + 0.3*math.Sin(m.clock*13) + 0.2*math.Sin(m.clock*31+1.3)
	return max(0, min(1, v))
}

// healthFraction, xpFraction and bossFraction are the HUD's fills from 0 to 1.
func (s hudState) healthFraction() float64 { return fraction(s.health, s.maxHealth, 1) }
func (s hudState) xpFraction() float64     { return fraction(s.xp, s.xpNeeded, 0) }
func (s hudState) bossFraction() float64   { return fraction(s.bossHealth, s.bossMaxHealth, 0) }

func fraction(n, of int, none float64) float64 {
	if of <= 0 {
		return none
	}
	return max(0, min(1, float64(n)/float64(of)))
}

// lowHealth reports whether the keeper is at or under the run's danger line.
func (s hudState) lowHealth() bool {
	return s.maxHealth > 0 && s.health > 0 && s.healthFraction() <= s.lowHealthAt
}

// hudLabel is what the canvas tells assistive technology about the HUD: the clock, health, level, kills and the boss.
func hudLabel(tr evstrings.Table, st hudState) string {
	label := tr.ArenaLabel
	if st.maxHealth <= 0 {
		return label
	}
	label += ", " + st.time + ", " + tr.Health(st.health, st.maxHealth) + ", " + tr.Level(st.level) + ", " + tr.Kills(st.kills)
	if st.bossActive {
		label += ", " + st.bossTitle(tr)
	}
	return label
}

// bossTitle is the boss bar's caption: the Hollow King's name and phase, or the Gloomwarden's name.
func (s hudState) bossTitle(tr evstrings.Table) string {
	if s.bossElite {
		return tr.EliteDisplayName(s.bossName)
	}
	return tr.BossDisplayName(s.bossName) + " · " + tr.BossPhaseLabel(s.bossPhase)
}

// hudPainter draws one frame of the HUD.
type hudPainter struct {
	d      *core.Drawing
	art    *hudArt
	icons  *art
	l      hudLayout
	st     hudState
	m      *hudMotion
	tr     evstrings.Table
	lang   string
	pushed bool
}

// drawHUD draws the HUD over the lit arena at the canvas's real size, so the kit's 2× pixels stay crisp. The pause button shows only while canPause, which a pause or settings screen over the run turns off.
func drawHUD(d *core.Drawing, l hudLayout, st hudState, m *hudMotion, kit *hudArt, icons *art, tr evstrings.Table, lang string, canPause, pushed bool) {
	if st.maxHealth <= 0 {
		return
	}
	p := hudPainter{d: d, art: kit, icons: icons, l: l, st: st, m: m, tr: tr, lang: lang, pushed: pushed}
	d.Save()
	d.SetImageSmoothing(true)
	p.drawHealth()
	p.drawXP()
	p.drawBadge()
	p.drawKills()
	p.drawSlots()
	p.drawTimer()
	if st.bossActive {
		p.drawBoss()
	}
	if canPause && st.phase == PhasePlaying {
		p.drawPause()
	}
	d.Restore()
}

func (p *hudPainter) drawHealth() {
	r := p.l.health
	frac := float32(p.m.health)
	low := p.st.lowHealth()
	bright := float32(1)
	if low {
		bright = 0.45 + 0.55*float32(p.m.flicker())
	}
	fill, okFill := p.art.image(hudHealthFill)
	frame, okFrame := p.art.image(hudHealthFrame)
	if !okFill || !okFrame {
		p.drawHealthFallback(r, frac, bright)
		return
	}
	sy := float32(fill.Size.Height) / r.h
	// The oil's glow behind the glass, which gutters when health runs low.
	cx, cy := r.center()
	glow := core.CanvasRadial(cx, cy, 0, cx, cy, r.w,
		core.GradientStop{Offset: 0, Color: core.RGBA(0xff9a3c, 0.35*bright*min(1, frac*3))},
		core.GradientStop{Offset: 1, Color: core.RGBA(0xff9a3c, 0)},
	)
	p.d.Save()
	p.d.SetFillGradient(glow).FillRect(r.x-r.w/2, r.y, r.w*2, r.h)
	p.d.Restore()
	cut := healthFillTop + (healthFillBottom-healthFillTop)*(1-frac)
	if frac > 0 {
		p.d.SetAlpha(0.55 + 0.45*bright)
		p.d.DrawImageFrom(fill, 0, cut, float32(fill.Size.Width), healthFillBottom-cut, r.x, r.y+cut/sy, r.w, (healthFillBottom-cut)/sy)
		p.d.SetAlpha(1)
	}
	p.d.DrawImageIn(frame, r.x, r.y, r.w, r.h)
	if p.m.flash > 0 {
		p.d.Save()
		p.d.SetComposite(core.CompositeLighter).SetAlpha(float32(p.m.flash) * 0.8)
		p.d.DrawImageIn(frame, r.x, r.y, r.w, r.h)
		p.d.Restore()
	}
}

func (p *hudPainter) drawHealthFallback(r rect, frac, bright float32) {
	s := p.l.scale
	glass := rect{r.x + 12*s, r.y + 20*s, r.w - 24*s, r.h - 40*s}
	p.d.SetFillColor(hudIron).BeginPath().RoundRect(glass.x-3*s, glass.y-3*s, glass.w+6*s, glass.h+6*s, 6*s).Fill()
	if frac > 0 {
		hh := glass.h * frac
		p.d.SetAlpha(0.55 + 0.45*bright)
		p.d.SetFillColor(hudOil).FillRect(glass.x, glass.y+glass.h-hh, glass.w, hh)
		p.d.SetAlpha(1)
	}
	p.d.SetStrokeColor(hudBronze).SetLineWidth(2*s).BeginPath().RoundRect(glass.x-3*s, glass.y-3*s, glass.w+6*s, glass.h+6*s, 6*s).Stroke()
	if p.m.flash > 0 {
		p.d.SetAlpha(float32(p.m.flash) * 0.6)
		p.d.SetFillColor(core.RGB(0xffffff)).FillRect(glass.x, glass.y, glass.w, glass.h)
		p.d.SetAlpha(1)
	}
}

func (p *hudPainter) drawXP() {
	r := p.l.xp
	frac := float32(p.m.xp)
	glow := float32(p.m.glow)
	fill, okFill := p.art.image(hudXPFill)
	frame, okFrame := p.art.image(hudXPFrame)
	if !okFill || !okFrame {
		s := p.l.scale
		p.d.SetFillColor(hudIron).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 6*s).Fill()
		p.d.SetFillColor(hudGold).FillRect(r.x+4*s, r.y+5*s, (r.w-8*s)*frac, r.h-10*s)
		p.d.SetStrokeColor(hudBronze).SetLineWidth(2*s).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 6*s).Stroke()
		if glow > 0 {
			p.d.SetAlpha(glow*0.6).SetFillColor(hudGold).FillRect(r.x+4*s, r.y+5*s, r.w-8*s, r.h-10*s).SetAlpha(1)
		}
		return
	}
	sx := float32(fill.Size.Width) / r.w
	p.d.DrawImageIn(frame, r.x, r.y, r.w, r.h)
	if frac > 0 {
		sw := (xpFillRight - xpFillLeft) * frac
		p.d.DrawImageFrom(fill, xpFillLeft, 0, sw, float32(fill.Size.Height), r.x+xpFillLeft/sx, r.y, sw/sx, r.h)
	}
	if glow > 0 {
		p.d.Save()
		p.d.SetComposite(core.CompositeLighter).SetAlpha(glow)
		p.d.SetShadow(core.RGBA(0xffb347, 0.9), 0, 0, 12*p.l.scale)
		p.d.DrawImageFrom(fill, xpFillLeft, 0, xpFillRight-xpFillLeft, float32(fill.Size.Height), r.x+xpFillLeft/sx, r.y, (xpFillRight-xpFillLeft)/sx, r.h)
		p.d.Restore()
	}
}

func (p *hudPainter) drawBadge() {
	r := p.l.badge
	if img, ok := p.art.image(hudLevelBadge); ok {
		p.d.DrawImageIn(img, r.x, r.y, r.w, r.h)
	} else {
		cx, cy := r.center()
		p.d.SetFillColor(hudBronze).BeginPath().MoveTo(cx, r.y).LineTo(r.x+r.w, cy).LineTo(cx, r.y+r.h).LineTo(r.x, cy).ClosePath().Fill()
	}
	cx, cy := r.center()
	p.drawNumber(strconv.Itoa(p.st.level), cx, cy, 20*p.l.scale, core.TextAlignCenter, false)
}

func (p *hudPainter) drawKills() {
	r := p.l.emblem
	if img, ok := p.art.image(hudKillEmblem); ok {
		p.d.DrawImageIn(img, r.x, r.y, r.w, r.h)
	} else {
		cx, cy := r.center()
		p.d.SetFillColor(hudIron).BeginPath().Ellipse(cx, cy, r.w/2.4, r.h/2.4).Fill()
	}
	p.drawNumber(strconv.Itoa(p.st.kills), p.l.killsX, p.l.killsY+p.l.killsH/2, p.l.killsH, core.TextAlignLeft, true)
}

func (p *hudPainter) drawSlots() {
	s := p.l.scale
	for i, slot := range p.st.weapons {
		r := p.l.slots[i]
		pop := float32(p.m.pops[i])
		if pop > 0 {
			// A new weapon lands with a little overshoot.
			grow := 1 + 0.3*pop*float32(math.Sin(float64(1-pop)*math.Pi))
			cx, cy := r.center()
			r = rect{cx - r.w*grow/2, cy - r.h*grow/2, r.w * grow, r.h * grow}
		}
		frameName := hudSlotEmpty
		switch {
		case slot.id == "":
		case slot.maxLevel > 0 && slot.level >= slot.maxLevel:
			frameName = hudSlotMaxed
		default:
			frameName = hudSlotFilled
		}
		if img, ok := p.art.image(frameName); ok {
			p.d.DrawImageIn(img, r.x, r.y, r.w, r.h)
		} else {
			color := hudIron
			if frameName == hudSlotMaxed {
				color = hudGold
			}
			p.d.SetFillColor(hudIron).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 6*s).Fill()
			p.d.SetStrokeColor(color).SetLineWidth(2*s).BeginPath().RoundRect(r.x+1*s, r.y+1*s, r.w-2*s, r.h-2*s, 6*s).Stroke()
		}
		if slot.id == "" {
			continue
		}
		icon := slotIconSide * r.w / slotSide
		cx, cy := r.center()
		if img, ok := p.icons.cardIcon(slot.id); ok {
			p.d.DrawImageIn(img, cx-icon/2, cy-icon/2, icon, icon)
		} else {
			p.d.SetFillColor(cardAccent(slot.id, offerWeapon)).BeginPath().Ellipse(cx, cy, icon/2.5, icon/2.5).Fill()
		}
		p.drawPips(p.l.slots[i], slot.level, slot.maxLevel)
	}
}

// drawPips lays maxLevel pips along a slot's bottom edge, lit up to level.
func (p *hudPainter) drawPips(r rect, level, maxLevel int) {
	if maxLevel <= 0 {
		return
	}
	side := min(pipSide*p.l.scale, r.w/float32(maxLevel))
	x := r.x + (r.w-side*float32(maxLevel))/2
	y := r.y + r.h - side/2
	lit, okLit := p.art.image(hudPipLit)
	unlit, okUnlit := p.art.image(hudPipUnlit)
	for i := range maxLevel {
		on := i < level
		px := x + float32(i)*side
		switch {
		case on && okLit:
			p.d.DrawImageIn(lit, px, y, side, side)
		case !on && okUnlit:
			p.d.DrawImageIn(unlit, px, y, side, side)
		default:
			c := pipEmpty
			if on {
				c = hudGold
			}
			h := side / 2
			p.d.SetFillColor(c).BeginPath().MoveTo(px+h, y).LineTo(px+side, y+h).LineTo(px+h, y+side).LineTo(px, y+h).ClosePath().Fill()
		}
	}
}

func (p *hudPainter) drawTimer() {
	r := p.l.timer
	if img, ok := p.art.image(hudTimer); ok {
		p.d.DrawImageIn(img, r.x, r.y, r.w, r.h)
	} else {
		p.d.SetFillColor(hudIron).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 8*p.l.scale).Fill()
		p.d.SetStrokeColor(hudBronze).SetLineWidth(2*p.l.scale).BeginPath().RoundRect(r.x, r.y, r.w, r.h, 8*p.l.scale).Stroke()
	}
	cx, cy := r.center()
	p.drawNumber(p.st.time, cx, cy, 26*p.l.scale, core.TextAlignCenter, false)
}

func (p *hudPainter) drawBoss() {
	r := p.l.boss
	if r.w <= 0 {
		return
	}
	frac := float32(p.m.boss)
	fill, okFill := p.art.image(hudBossFill)
	frame, okFrame := p.art.image(hudBossFrame)
	if okFill && okFrame {
		sx := float32(fill.Size.Width) / r.w
		p.d.DrawImageIn(frame, r.x, r.y, r.w, r.h)
		if frac > 0 {
			sw := (bossFillRight - bossFillLeft) * frac
			p.d.DrawImageFrom(fill, bossFillLeft, 0, sw, float32(fill.Size.Height), r.x+bossFillLeft/sx, r.y, sw/sx, r.h)
		}
	} else {
		trough := rect{r.x + r.w*0.12, r.y + r.h*0.36, r.w * 0.76, r.h * 0.3}
		p.d.SetFillColor(hudIron).BeginPath().RoundRect(trough.x-3, trough.y-3, trough.w+6, trough.h+6, 6).Fill()
		p.d.SetFillColor(hudBossRed).FillRect(trough.x, trough.y, trough.w*frac, trough.h)
		p.d.SetStrokeColor(hudBronze).SetLineWidth(2).BeginPath().RoundRect(trough.x-3, trough.y-3, trough.w+6, trough.h+6, 6).Stroke()
	}
	cx, cy := r.center()
	font := core.Font{Size: core.Points(max(10, 14*r.w/bossW)), Weight: core.FontWeightBold, Language: p.lang}
	p.d.SetTextAlign(core.TextAlignCenter).SetTextBaseline(core.BaselineMiddle)
	p.d.SetStrokeColor(hudOutline).SetLineWidth(3).StrokeText(p.st.bossTitle(p.tr), cx, cy, font)
	p.d.SetFillColor(hudIvory).FillText(p.st.bossTitle(p.tr), cx, cy, font)
}

func (p *hudPainter) drawPause() {
	r := p.l.pause
	name := hudPause
	if p.pushed {
		name = hudPausePushed
	}
	if img, ok := p.art.image(name); ok {
		p.d.DrawImageIn(img, r.x, r.y, r.w, r.h)
		return
	}
	s := p.l.scale
	cx, cy := r.center()
	rad := r.w / 2
	p.d.BeginPath()
	for i := range 8 {
		a := float64(i)*math.Pi/4 + math.Pi/8
		x, y := cx+rad*float32(math.Cos(a)), cy+rad*float32(math.Sin(a))
		if i == 0 {
			p.d.MoveTo(x, y)
		} else {
			p.d.LineTo(x, y)
		}
	}
	p.d.ClosePath()
	fillColor := hudIron
	if p.pushed {
		fillColor = core.RGBA(0x3a2a1a, 0.95)
	}
	p.d.SetFillColor(fillColor).Fill()
	p.d.SetStrokeColor(hudBronze).SetLineWidth(2 * s).Stroke()
	bw, bh := 6*s, 20*s
	p.d.SetFillColor(hudIvory).FillRect(cx-bw-3*s, cy-bh/2, bw, bh).FillRect(cx+3*s, cy-bh/2, bw, bh)
}

// drawNumber draws text such as "12" or "03:41" in the kit's painted numerals, h points tall and centred on y, or in canvas text when the numerals are missing. times puts the kit's × before it.
func (p *hudPainter) drawNumber(text string, x, y, h float32, align core.TextAlign, times bool) {
	if !p.art.numeralsReady() {
		if times {
			text = "×" + text
		}
		font := core.Font{Size: core.Points(h * 0.8), Weight: core.FontWeightBold}
		p.d.SetTextAlign(align).SetTextBaseline(core.BaselineMiddle)
		p.d.SetStrokeColor(hudOutline).SetLineWidth(max(2, h/8)).StrokeText(text, x, y, font)
		p.d.SetFillColor(hudIvory).FillText(text, x, y, font)
		return
	}
	k := h / digitH
	width := float32(0)
	if times {
		width += digitW * k
	}
	for _, c := range text {
		if c == ':' {
			width += colonW * k
		} else {
			width += digitW * k
		}
	}
	switch align {
	case core.TextAlignCenter:
		x -= width / 2
	case core.TextAlignRight, core.TextAlignEnd:
		x -= width
	}
	top := y - h/2
	if times {
		p.d.DrawImageIn(p.art.times, x, top, digitW*k, h)
		x += digitW * k
	}
	for _, c := range text {
		switch {
		case c == ':':
			p.d.DrawImageIn(p.art.colon, x, top, colonW*k, h)
			x += colonW * k
		case c >= '0' && c <= '9':
			p.d.DrawImageIn(p.art.digits[c-'0'], x, top, digitW*k, h)
			x += digitW * k
		}
	}
}

func (a *hudArt) numeralsReady() bool { return a != nil && a.numerals }
