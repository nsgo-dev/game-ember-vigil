package main

import (
	"bytes"
	"embed"
	"image"
	imagedraw "image/draw"
	"image/png"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/game/packages/game/random"
	"github.com/nsgo-dev/game/packages/game/sprite"
	"github.com/nsgo-dev/nsgo/core"
)

//go:embed assets/sprites/*.png
var embeddedSprites embed.FS

//go:embed assets/icons/*.png
var embeddedIcons embed.FS

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

// sheetActor holds one actor's sliced animations and logical scale from assets/sprites/.
// When loaded is false, the renderer falls back to the procedural atlas frame.
type sheetActor struct {
	loaded           bool
	scale            float32
	defaultFacesLeft bool
	idle             sprite.Animation
	move             sprite.Animation
	round            time.Duration
}

// sheetFrames holds every actor's loaded sprite-sheet animations by field so drawing never allocates or hashes strings.
type sheetFrames struct {
	keeper      sheetActor
	mite        sheetActor
	moth        sheetActor
	husk        sheetActor
	thornling   sheetActor
	wisp        sheetActor
	shade       sheetActor
	gloomwarden sheetActor
	hollowking  sheetActor
	ember       sheetActor
	bolt        sheetActor
	halo        sheetActor
	thorn       sheetActor
	bone        sheetActor
	cache       sheetActor
}

// sheetSpec describes one uniform-grid PNG sprite sheet under assets/sprites/.
type sheetSpec struct {
	name             string
	file             string
	rows             int
	cols             int
	idleFrameTime    time.Duration
	moveFrameTime    time.Duration
	worldHeight      float32
	defaultFacesLeft bool
	target           func(*sheetFrames) *sheetActor
}

// sheetSpecs lists every stylized 2D sprite sheet the loader recognizes, including #14 sheets that fall back cleanly until added.
var sheetSpecs = []sheetSpec{
	{
		name: "keeper", file: "assets/sprites/keeper.png",
		rows: 2, cols: 4, idleFrameTime: 180 * time.Millisecond, moveFrameTime: 110 * time.Millisecond,
		worldHeight: 28, defaultFacesLeft: true,
		target: func(s *sheetFrames) *sheetActor { return &s.keeper },
	},
	{
		name: "mite", file: "assets/sprites/mite.png",
		rows: 1, cols: 4, moveFrameTime: 110 * time.Millisecond,
		worldHeight: 20,
		target:      func(s *sheetFrames) *sheetActor { return &s.mite },
	},
	{
		name: "moth", file: "assets/sprites/moth.png",
		rows: 1, cols: 4, moveFrameTime: 90 * time.Millisecond,
		worldHeight: 20,
		target:      func(s *sheetFrames) *sheetActor { return &s.moth },
	},
	{
		name: "husk", file: "assets/sprites/husk.png",
		rows: 1, cols: 4, moveFrameTime: 150 * time.Millisecond,
		worldHeight: 30,
		target:      func(s *sheetFrames) *sheetActor { return &s.husk },
	},
	{
		name: "thornling", file: "assets/sprites/thornling.png",
		rows: 1, cols: 4, moveFrameTime: 120 * time.Millisecond,
		worldHeight: 24,
		target:      func(s *sheetFrames) *sheetActor { return &s.thornling },
	},
	{
		name: "wisp", file: "assets/sprites/wisp.png",
		rows: 1, cols: 4, moveFrameTime: 100 * time.Millisecond,
		worldHeight: 22,
		target:      func(s *sheetFrames) *sheetActor { return &s.wisp },
	},
	{
		name: "shade", file: "assets/sprites/shade.png",
		rows: 1, cols: 4, moveFrameTime: 120 * time.Millisecond,
		worldHeight: 26,
		target:      func(s *sheetFrames) *sheetActor { return &s.shade },
	},
	{
		name: "gloomwarden", file: "assets/sprites/gloomwarden.png",
		rows: 1, cols: 4, moveFrameTime: 140 * time.Millisecond,
		worldHeight: 38,
		target:      func(s *sheetFrames) *sheetActor { return &s.gloomwarden },
	},
	{
		name: "hollowking", file: "assets/sprites/hollowking.png",
		rows: 1, cols: 4, moveFrameTime: 150 * time.Millisecond,
		worldHeight: 52,
		target:      func(s *sheetFrames) *sheetActor { return &s.hollowking },
	},
	{
		name: "ember", file: "assets/sprites/ember.png",
		rows: 1, cols: 4, moveFrameTime: 120 * time.Millisecond,
		worldHeight: 10,
		target:      func(s *sheetFrames) *sheetActor { return &s.ember },
	},
	{
		name: "bolt", file: "assets/sprites/bolt.png",
		rows: 1, cols: 4, moveFrameTime: 70 * time.Millisecond,
		worldHeight: 14,
		target:      func(s *sheetFrames) *sheetActor { return &s.bolt },
	},
	{
		name: "halo", file: "assets/sprites/halo.png",
		rows: 1, cols: 4, moveFrameTime: 100 * time.Millisecond,
		worldHeight: 14,
		target:      func(s *sheetFrames) *sheetActor { return &s.halo },
	},
	{
		name: "thorn", file: "assets/sprites/thorn.png",
		rows: 1, cols: 4, moveFrameTime: 80 * time.Millisecond,
		worldHeight: 12,
		target:      func(s *sheetFrames) *sheetActor { return &s.thorn },
	},
	{
		name: "bone", file: "assets/sprites/bone.png",
		rows: 1, cols: 4, moveFrameTime: 90 * time.Millisecond,
		worldHeight: 14,
		target:      func(s *sheetFrames) *sheetActor { return &s.bone },
	},
	{
		name: "cache", file: "assets/sprites/cache.png",
		rows: 1, cols: 4, moveFrameTime: 130 * time.Millisecond,
		worldHeight: 16,
		target:      func(s *sheetFrames) *sheetActor { return &s.cache },
	},
}

// art holds the procedural fallback atlas, the packed stylized sprite-sheet atlas, and embedded upgrade card icons.
type art struct {
	atlas      *sprite.Atlas
	frames     frames
	pixels     map[string]*pixels
	sheetAtlas *sprite.Atlas
	sheets     sheetFrames
	icons      map[string]core.ImageSource
}

// newArt builds the procedural fallback atlas and loads all embedded sprite sheets and upgrade card icons.
func newArt() *art {
	return newArtFromFS(embeddedSprites, embeddedIcons)
}

// newArtFromFS builds the procedural fallback atlas and loads any available sprite sheets and icons from spriteFS and iconFS.
// Missing or unreadable files are skipped so callers always get a valid fallback.
func newArtFromFS(spriteFS, iconFS fs.FS) *art {
	ras := make([]*pixels, len(pictures))
	for i, p := range pictures {
		ras[i] = rasterize(p)
	}
	f, width, height := packFrames(ras)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range pictures {
		ras[i].drawInto(img, int(f[i].X), int(f[i].Y))
	}

	a := &art{
		atlas:  &sprite.Atlas{Image: core.ImageSource{Pixels: img, Version: 1}, Frames: map[string]sprite.Frame{}},
		pixels: make(map[string]*pixels, len(pictures)),
		icons:  map[string]core.ImageSource{},
	}
	for i, p := range pictures {
		a.atlas.Frames[p.name] = f[i]
		a.pixels[p.name] = ras[i]
	}
	a.frames = framesOf(a.atlas.Frames)
	a.loadSheets(spriteFS)
	a.loadIcons(iconFS)
	return a
}

// cardIcon returns the embedded PNG icon for an upgrade card id (converting hyphens to underscores), or false when absent.
func (a *art) cardIcon(id string) (core.ImageSource, bool) {
	if a == nil || len(a.icons) == 0 {
		return core.ImageSource{}, false
	}
	src, ok := a.icons[strings.ReplaceAll(id, "-", "_")]
	return src, ok
}

// decodedSheet is one sprite sheet decoded into premultiplied RGBA and validated against its uniform grid.
type decodedSheet struct {
	spec         sheetSpec
	img          *image.RGBA
	cellW, cellH int
}

// loadSheets decodes every available sprite sheet from fsys, slices its uniform grid into frames, and packs all frames into one atlas with transparent padding between cells.
func (a *art) loadSheets(fsys fs.FS) {
	if fsys == nil {
		return
	}
	var loaded []decodedSheet
	for _, spec := range sheetSpecs {
		raw, err := fs.ReadFile(fsys, spec.file)
		if err != nil {
			continue
		}
		rgba, err := decodePNGToRGBA(raw)
		if err != nil {
			continue
		}
		b := rgba.Bounds()
		w, h := b.Dx(), b.Dy()
		if spec.cols <= 0 || spec.rows <= 0 || w <= 0 || h <= 0 || w%spec.cols != 0 || h%spec.rows != 0 {
			continue
		}
		loaded = append(loaded, decodedSheet{
			spec:  spec,
			img:   rgba,
			cellW: w / spec.cols,
			cellH: h / spec.rows,
		})
	}
	if len(loaded) == 0 {
		return
	}

	const (
		sheetAtlasWidth = 512
		sheetGap        = 2
	)
	type placement struct {
		sheetIdx int
		row, col int
		x, y     int
	}
	var places []placement
	x, y, shelf := 0, 0, 0
	atlasW, atlasH := 0, 0
	for si, ds := range loaded {
		for row := 0; row < ds.spec.rows; row++ {
			for col := 0; col < ds.spec.cols; col++ {
				if x > 0 && x+ds.cellW > sheetAtlasWidth {
					x, y = 0, y+shelf+sheetGap
					shelf = 0
				}
				places = append(places, placement{sheetIdx: si, row: row, col: col, x: x, y: y})
				x += ds.cellW + sheetGap
				shelf = max(shelf, ds.cellH)
				atlasW = max(atlasW, x-sheetGap)
				atlasH = max(atlasH, y+shelf)
			}
		}
	}

	packed := image.NewRGBA(image.Rect(0, 0, atlasW, atlasH))
	atlas := &sprite.Atlas{
		Image:      core.ImageSource{Pixels: packed, Version: 1},
		Frames:     make(map[string]sprite.Frame, len(places)),
		Animations: make(map[string]sprite.Animation, len(loaded)*2),
	}

	// Copy each sliced cell into the packed atlas and index its Frame by sheet/row/col.
	gridFrames := make([][][]sprite.Frame, len(loaded))
	for si, ds := range loaded {
		gridFrames[si] = make([][]sprite.Frame, ds.spec.rows)
		for row := 0; row < ds.spec.rows; row++ {
			gridFrames[si][row] = make([]sprite.Frame, ds.spec.cols)
		}
	}
	for _, pl := range places {
		ds := loaded[pl.sheetIdx]
		srcMin := image.Pt(pl.col*ds.cellW, pl.row*ds.cellH)
		dstRect := image.Rect(pl.x, pl.y, pl.x+ds.cellW, pl.y+ds.cellH)
		imagedraw.Draw(packed, dstRect, ds.img, srcMin, imagedraw.Src)

		name := ds.spec.name + "/" + strconv.Itoa(pl.row) + "/" + strconv.Itoa(pl.col)
		fr := sprite.Frame{
			Name:   name,
			X:      float32(pl.x),
			Y:      float32(pl.y),
			W:      float32(ds.cellW),
			H:      float32(ds.cellH),
			Width:  float32(ds.cellW),
			Height: float32(ds.cellH),
			PivotX: float32(ds.cellW) / 2,
			PivotY: float32(ds.cellH) / 2,
		}
		atlas.Frames[name] = fr
		gridFrames[pl.sheetIdx][pl.row][pl.col] = fr
	}

	for si, ds := range loaded {
		spec := ds.spec
		actor := sheetActor{
			loaded:           true,
			scale:            spec.worldHeight / float32(ds.cellH),
			defaultFacesLeft: spec.defaultFacesLeft,
		}
		if spec.rows >= 2 {
			idleDur := spec.idleFrameTime
			if idleDur <= 0 {
				idleDur = spec.moveFrameTime
			}
			actor.idle = buildClip(gridFrames[si][0], idleDur)
			actor.move = buildClip(gridFrames[si][1], spec.moveFrameTime)
			actor.round = spec.moveFrameTime * time.Duration(spec.cols)
			atlas.Animations[spec.name+":idle"] = actor.idle
			atlas.Animations[spec.name+":walk"] = actor.move
		} else {
			clip := buildClip(gridFrames[si][0], spec.moveFrameTime)
			actor.idle = clip
			actor.move = clip
			actor.round = spec.moveFrameTime * time.Duration(spec.cols)
			atlas.Animations[spec.name] = clip
		}
		*spec.target(&a.sheets) = actor
	}
	a.sheetAtlas = atlas
}

// buildClip creates a looping sprite.Animation from a row of uniform frames.
func buildClip(row []sprite.Frame, frameDuration time.Duration) sprite.Animation {
	af := make([]sprite.AnimationFrame, len(row))
	for i, fr := range row {
		af[i] = sprite.AnimationFrame{Frame: fr, Duration: frameDuration}
	}
	return sprite.Animation{Frames: af, Loop: true}
}

// loadIcons decodes every PNG under assets/icons/ in fsys into premultiplied RGBA ImageSources keyed by filename stem.
func (a *art) loadIcons(fsys fs.FS) {
	if fsys == nil {
		return
	}
	entries, err := fs.ReadDir(fsys, "assets/icons")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		raw, err := fs.ReadFile(fsys, "assets/icons/"+e.Name())
		if err != nil {
			continue
		}
		rgba, err := decodePNGToRGBA(raw)
		if err != nil {
			continue
		}
		b := rgba.Bounds()
		stem := strings.TrimSuffix(e.Name(), ".png")
		a.icons[stem] = core.ImageSource{
			Pixels:  rgba,
			Version: 1,
			Size:    core.Size{Width: core.Points(b.Dx()), Height: core.Points(b.Dy())},
		}
	}
}

// decodePNGToRGBA decodes PNG bytes into a premultiplied *image.RGBA suitable for core.ImageSource.Pixels.
func decodePNGToRGBA(raw []byte) (*image.RGBA, error) {
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if rgba, ok := src.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba, nil
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	imagedraw.Draw(dst, dst.Bounds(), src, b.Min, imagedraw.Src)
	return dst, nil
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
