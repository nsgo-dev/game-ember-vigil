package main

import (
	"image"
	"math"
	"strconv"

	"github.com/nsgo-dev/game/packages/game/random"
	"github.com/nsgo-dev/game/packages/game/sprite"
	"github.com/nsgo-dev/nsgo/core"
)

// The palette's sixteen indices, from the night's ink to the rust of the Cinderwood's floor. A picture names an index and never a colour.
const (
	palInk uint8 = iota
	palNight
	palDusk
	palSlate
	palAsh
	palMist
	palBone
	palPine
	palMoss
	palEmber
	palFlame
	palGold
	palScarlet
	palBlood
	palViolet
	palRust
)

// palette is the whole game's colour: index 0 is ink and the rest light it.
var palette = [16]core.Color{
	palInk:     core.RGB(0x0b0d17),
	palNight:   core.RGB(0x151b2b),
	palDusk:    core.RGB(0x27314a),
	palSlate:   core.RGB(0x3b4a66),
	palAsh:     core.RGB(0x55637f),
	palMist:    core.RGB(0x8a93a8),
	palBone:    core.RGB(0xd7dce8),
	palPine:    core.RGB(0x3f6b4a),
	palMoss:    core.RGB(0x24382b),
	palEmber:   core.RGB(0xff7a1a),
	palFlame:   core.RGB(0xffb347),
	palGold:    core.RGB(0xffd166),
	palScarlet: core.RGB(0xa3341a),
	palBlood:   core.RGB(0x6b1a1a),
	palViolet:  core.RGB(0x5b4a7f),
	palRust:    core.RGB(0x5a3a2a),
}

// picture is one sprite's pixel art: each row a palette index per pixel written as a hex digit, with '.' or '0' for a clear pixel.
type picture struct {
	name string
	rows []string
}

// pictures are the atlas's pictures, in the order the atlas packs them. Nothing here is an image file: the atlas is built from these rows at startup.
var pictures = []picture{
	{name: "keeper", rows: []string{
		"....1111....",
		"...133331...",
		"..13333331..",
		"..13566531..",
		"..13599531..",
		"..13355331..",
		"...133331...",
		"..13344331..",
		".1333443331.",
		".1333aa3331.",
		".1222aa2221.",
		".1222333221.",
		".1222333221.",
		"..12222221..",
		"...11..11...",
		"...11..11...",
	}},
	{name: "mite", rows: []string{
		"............",
		"...112211...",
		"..12222221..",
		".1222222221.",
		".1229929921.",
		".1222222221.",
		".1212222121.",
		"..12222221..",
		"..1.1221.1..",
		".1..1..1..1.",
		"1..1....1..1",
		"............",
	}},
	{name: "moth", rows: []string{
		"............",
		"..e......e..",
		".eee.11.eee.",
		"eee..11..eee",
		"eee.1991.eee",
		".eee1111eee.",
		"..ee1111ee..",
		".eee.11.eee.",
		"eee..11..eee",
		".eee....eee.",
		"..e......e..",
		"............",
	}},
	{name: "husk", rows: []string{
		"......................",
		".......33333333.......",
		".....333333333333.....",
		"....33334444443333....",
		"...3344444444444433...",
		"...3355555555555533...",
		"...3355995555995533...",
		"...3355555555555533...",
		"...3335555555553333...",
		"...3333555555553333...",
		"....33225555552233....",
		"....22225555552222....",
		".....222222222222.....",
		"....2222......2222....",
		"....2222......2222....",
		"......................",
	}},
	{name: "ember", rows: []string{
		"..bb..",
		".baab.",
		"baaaab",
		"baaaab",
		".baab.",
		"..bb..",
	}},
	{name: "bolt", rows: []string{
		"....bbbb",
		"...baaab",
		"..baaaab",
		".baaaaab",
		"9baaaaab",
		"99baaaab",
		"999baab.",
		"9999....",
	}},
	{name: "halo", rows: []string{
		"..9999..",
		".9aaaa9.",
		"9aabbaa9",
		"9ab66ba9",
		"9ab66ba9",
		"9aabbaa9",
		".9aaaa9.",
		"..9999..",
	}},
	{name: "spark", rows: []string{
		".66.",
		"6bb6",
		"6bb6",
		".66.",
	}},
	{name: "puff", rows: []string{
		"..55..",
		".5445.",
		"544445",
		"544445",
		".5445.",
		"..55..",
	}},
	{name: "trail", rows: []string{
		"b9",
		"9.",
	}},
	{name: "grass", rows: []string{
		"..7..7..",
		"..7.87..",
		".77.87.7",
		".878887.",
		"..8888..",
		"...88...",
	}},
	{name: "fern", rows: []string{
		"..8....8..",
		"..87..78..",
		".78777787.",
		".77777777.",
		"..777777..",
		"...7777...",
		"....77....",
		"....77....",
	}},
	{name: "stone", rows: []string{
		"...2222...",
		"..2333332.",
		".233333332",
		".233333332",
		"..2333332.",
		"...2222...",
	}},
	{name: "mushroom", rows: []string{
		"..9999..",
		".9dddd9.",
		"9dddddd9",
		".9dddd9.",
		"..3773..",
		"..3773..",
		"...33...",
	}},
	{name: "twig", rows: []string{
		"..fff.......",
		".fffff......",
		"...fffffff..",
		".......ffff.",
	}},
	{name: "thornling", rows: []string{
		"......77......",
		"....778877....",
		"..7.877778.7..",
		".778777777877.",
		"..877c77c778..",
		".777777777777.",
		"778777ee777877",
		".787777777787.",
		"..8778888778..",
		"...88877888...",
		"..88.8778.88..",
		".8...8..8...8.",
		"..............",
		"..............",
	}},
	{name: "wisp", rows: []string{
		"....eeee....",
		"..ee6666ee..",
		".e66bbbb66e.",
		".e6b6666b6e.",
		"e6b6a66a6b6e",
		"e6b666666b6e",
		".e6bbbbbb6e.",
		".ee666666ee.",
		"..ee.ee.ee..",
		"...e.ee.e...",
		".....ee.....",
		"............",
	}},
	{name: "shade", rows: []string{
		"....111111....",
		"...11eeee11...",
		"..11eeeeee11..",
		"..1ee1111ee1..",
		".11e166166e11.",
		".11e111111e11.",
		".11eeeeeeee11.",
		".11ee5555ee11.",
		"..11eeeeee11..",
		"..11ee11ee11..",
		"..11e1..1e11..",
		".11ee....ee11.",
		".1e1......1e1.",
		".1..........1.",
		"..............",
		"..............",
	}},
	{name: "gloomwarden", rows: []string{
		"......b....b........",
		".....bbb..bbb.......",
		"....11cccccc11......",
		"...11cccccccc11.....",
		"..11223333332211....",
		"..12233333333221....",
		".1123344444433211...",
		".1233499449943321...",
		".1233444444443321...",
		".1233344cc4433321...",
		".1223333cc3333221...",
		"..12233333333221....",
		"..11223333332211....",
		"...112222222211.....",
		"...122222222221.....",
		"...12221..12221.....",
		"...1221....1221.....",
		"...111......111.....",
		"....................",
		"....................",
	}},
	{name: "hollowking", rows: []string{
		"....66..............66....",
		"....666....6666....666....",
		".....666..66bb66..666.....",
		"......666666bb666666......",
		"......66666666666666......",
		".....1166666666666611.....",
		"....116655666666556611....",
		"....166555666666555661....",
		"...11665995666659956611...",
		"...16655995666659955661...",
		"...16655555666655555661...",
		"....166665555555566661....",
		"....116651651651656611....",
		".....1115555555555111.....",
		"...1122211cccccc1122211...",
		"..11223321cccccc12332211..",
		"..122333211cccc112333221..",
		".11233332211cc11223333211.",
		".122333322116611223333221.",
		".122233222116611222332221.",
		"..1222222211661122222221..",
		"...11222221111112222211...",
		"....1122221....1222211....",
		".....11221......12211.....",
		"......111........111......",
		"..........................",
	}},
	{name: "thorn", rows: []string{
		"..cc..",
		".ceec.",
		"cebbec",
		"cebbec",
		".ceec.",
		"..cc..",
	}},
	{name: "bone", rows: []string{
		"..666...",
		".66666..",
		"6655566.",
		"665c566.",
		"6655566.",
		".66666..",
		"..666...",
		"........",
	}},
	{name: "cache", rows: []string{
		"...bbbb...",
		"..b7777b..",
		".b776677b.",
		"b776bb677b",
		"b76bbbb67b",
		"b76bbbb67b",
		"b776bb677b",
		".b776677b.",
		"..b7777b..",
		"...bbbb...",
	}},
}

// pixels is a picture rasterized to the palette, which both the atlas and the ground draw.
type pixels struct {
	w, h  int
	color []core.Color
}

// frames are the atlas's frames by name, so drawing reads a field instead of looking a string up for every sprite.
type frames struct {
	keeper, mite, moth, husk              sprite.Frame
	thornling, wisp, shade                sprite.Frame
	gloomwarden, hollowking               sprite.Frame
	ember, bolt, halo, thorn, bone, cache sprite.Frame
	spark, puff, trail                    sprite.Frame
	grass, fern, stone                    sprite.Frame
	mushroom, twig                        sprite.Frame
}

// decor is the pictures the ground scatters, in the order it picks from them.
func (f frames) decor() []sprite.Frame {
	return []sprite.Frame{f.grass, f.grass, f.fern, f.stone, f.twig, f.mushroom}
}

// art is the game's generated pixel art: one atlas for every sprite batch, and the pictures the ground paints from.
type art struct {
	atlas  *sprite.Atlas
	frames frames
	pixels map[string]*pixels
}

// newArt draws every picture into one atlas image, so startup costs a few thousand pixels and the repository carries no sprite sheet.
func newArt() *art {
	ras := make([]*pixels, len(pictures))
	for i, p := range pictures {
		ras[i] = rasterize(p)
	}
	frames, width, height := packFrames(ras)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range pictures {
		ras[i].drawInto(img, int(frames[i].X), int(frames[i].Y))
	}

	a := &art{
		atlas:  &sprite.Atlas{Image: core.ImageSource{Pixels: img, Version: 1}, Frames: map[string]sprite.Frame{}},
		pixels: make(map[string]*pixels, len(pictures)),
	}
	for i, p := range pictures {
		a.atlas.Frames[p.name] = frames[i]
		a.pixels[p.name] = ras[i]
	}
	a.frames = framesOf(a.atlas.Frames)
	return a
}

// framesOf takes the frames the arena draws by name; a missing picture is a programming error, not a runtime state.
func framesOf(m map[string]sprite.Frame) frames {
	get := func(name string) sprite.Frame {
		f, ok := m[name]
		if !ok {
			panic("embervigil: the atlas has no picture named " + name)
		}
		return f
	}
	return frames{
		keeper:      get("keeper"),
		mite:        get("mite"),
		moth:        get("moth"),
		husk:        get("husk"),
		thornling:   get("thornling"),
		wisp:        get("wisp"),
		shade:       get("shade"),
		gloomwarden: get("gloomwarden"),
		hollowking:  get("hollowking"),
		ember:       get("ember"),
		bolt:        get("bolt"),
		halo:        get("halo"),
		thorn:       get("thorn"),
		bone:        get("bone"),
		cache:       get("cache"),
		spark:       get("spark"),
		puff:        get("puff"),
		trail:       get("trail"),
		grass:       get("grass"),
		fern:        get("fern"),
		stone:       get("stone"),
		mushroom:    get("mushroom"),
		twig:        get("twig"),
	}
}

// rasterize turns a picture's rows into palette colours, refusing rows of different widths and digits the palette does not have. A '.' stays clear, so what is under it shows through.
func rasterize(p picture) *pixels {
	if len(p.rows) == 0 || len(p.rows[0]) == 0 {
		panic("embervigil: picture " + p.name + " has no pixels")
	}
	w, h := len(p.rows[0]), len(p.rows)
	out := &pixels{w: w, h: h, color: make([]core.Color, w*h)}
	for y, row := range p.rows {
		if len(row) != w {
			panic("embervigil: picture " + p.name + " row " + strconv.Itoa(y) + " is " + strconv.Itoa(len(row)) + " pixels wide, want " + strconv.Itoa(w))
		}
		for x := 0; x < w; x++ {
			switch c := row[x]; {
			case c == '.':
			case c >= '0' && c <= '9':
				out.color[y*w+x] = palette[c-'0']
			case c >= 'a' && c <= 'f':
				out.color[y*w+x] = palette[10+c-'a']
			default:
				panic("embervigil: picture " + p.name + " has the unknown pixel " + string(c))
			}
		}
	}
	return out
}

// drawInto copies p into img at x, y, keeping what is under its clear pixels and cutting what falls outside the image.
func (p *pixels) drawInto(img *image.RGBA, x, y int) {
	bounds := img.Bounds()
	for py := 0; py < p.h; py++ {
		dy := y + py
		if dy < bounds.Min.Y || dy >= bounds.Max.Y {
			continue
		}
		for px := 0; px < p.w; px++ {
			dx := x + px
			if dx < bounds.Min.X || dx >= bounds.Max.X {
				continue
			}
			if c := p.color[py*p.w+px]; c&0xff != 0 {
				setPixel(img, dx, dy, pixel(c))
			}
		}
	}
}

// pixel unpacks an sRGB colour into the premultiplied bytes an image.RGBA holds.
func pixel(c core.Color) [4]byte {
	a := uint32(c & 0xff)
	if a == 0 {
		return [4]byte{}
	}
	return [4]byte{
		uint8(uint32(c>>24) * a / 255),
		uint8(uint32(c>>16) * a / 255),
		uint8(uint32(c>>8) * a / 255),
		uint8(a),
	}
}

// setPixel writes px at x, y of img, whose caller has checked the point lies inside it.
func setPixel(img *image.RGBA, x, y int, px [4]byte) {
	i := img.PixOffset(x, y)
	copy(img.Pix[i:i+4], px[:])
}

// packFrames lays the pictures out in atlas rows no wider than atlasWidth, leaving a pixel between them so no frame samples its neighbour, and returns each frame with its picture centered on the frame's pivot.
func packFrames(ras []*pixels) ([]sprite.Frame, int, int) {
	const atlasWidth = 128
	out := make([]sprite.Frame, len(ras))
	x, y, shelf := 0, 0, 0
	width, height := 0, 0
	for i, p := range ras {
		if x > 0 && x+p.w > atlasWidth {
			x, y = 0, y+shelf+frameGap
			shelf = 0
		}
		out[i] = sprite.Frame{
			Name: pictures[i].name,
			X:    float32(x), Y: float32(y),
			W: float32(p.w), H: float32(p.h),
			Width: float32(p.w), Height: float32(p.h),
			PivotX: float32(p.w) / 2, PivotY: float32(p.h) / 2,
		}
		x += p.w + frameGap
		shelf = max(shelf, p.h)
		width = max(width, x)
		height = max(height, y+shelf)
	}
	return out, width, height
}

// frameGap is the clear pixels the atlas leaves between two pictures.
const frameGap = 1

// The ground's look: the size of a soil block, and how many pieces of decor the run's cosmetic stream scatters over the arena.
const (
	soilBlock  = 4
	decorSpots = 900
)

// newGround paints the arena's ground once per run: mottled soil, and the decor the run's cosmetic stream scatters over it. Two runs with one seed paint the same ground.
func (a *art) newGround(seed uint64, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y += soilBlock {
		for x := 0; x < width; x += soilBlock {
			soil := palette[palNight]
			switch mottle(seed, x/soilBlock, y/soilBlock) & 31 {
			case 0:
				soil = palette[palMoss]
			case 1:
				soil = palette[palInk]
			}
			fillInto(img, x, y, soilBlock, soilBlock, soil)
		}
	}

	decor := a.frames.decor()
	s := random.NewStreams(seed).Stream("ground")
	for range decorSpots {
		p := a.pixels[pictures[decorChoice(s, len(decor))].name]
		x, y := s.Intn(width)-p.w/2, s.Intn(height)-p.h/2
		p.drawInto(img, x, y)
	}
	return img
}

// decorChoice picks a decor picture from the ground stream, weighted so grass and ferns cover more of the forest floor than its stones.
func decorChoice(s *random.Stream, n int) int {
	return s.WeightedIndex(decorWeights[:n])
}

// decorWeights follow frames.decor's order: grass twice, fern, stone, twig, mushroom, so the floor reads as a forest rather than a quarry.
var decorWeights = []float64{5, 5, 3, 1.2, 0.9, 0.6}

// fillInto paints a solid block into img, clipped to its bounds.
func fillInto(img *image.RGBA, x, y, w, h int, c core.Color) {
	if c&0xff == 0 {
		return
	}
	p, b := pixel(c), img.Bounds()
	for py := max(y, b.Min.Y); py < min(y+h, b.Max.Y); py++ {
		for px := max(x, b.Min.X); px < min(x+w, b.Max.X); px++ {
			setPixel(img, px, py, p)
		}
	}
}

// mottle hashes a soil block's place and the run's seed to a number, which picks its shade: the ground is fixed per seed and costs no stream.
func mottle(seed uint64, x, y int) uint64 {
	h := seed ^ 0x9e3779b97f4a7c15
	h ^= uint64(x) * 0x9e3779b97f4a7c15
	h = (h ^ (h >> 30)) * 0xbf58476d1ce4e5b9
	h ^= uint64(y) * 0x94d049bb133111eb
	h = (h ^ (h >> 27)) * 0x94d049bb133111eb
	return h ^ (h >> 31)
}

// clampLength keeps v inside [lo, hi], or at lo when the range is empty because the world is smaller than the view.
func clampLength(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	return math.Min(math.Max(v, lo), hi)
}
