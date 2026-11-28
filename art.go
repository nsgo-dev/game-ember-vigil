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

//go:embed assets/environment/*.png
var embeddedEnvironment embed.FS

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
	{name: "chest", rows: []string{
		"..11111111..",
		".1ffffffff1.",
		"1ffffffffff1",
		"1bbbbbbbbbb1",
		"1ffffbbffff1",
		"1fffbaabfff1",
		"1ffffbbffff1",
		"1ffffffffff1",
		"1bbbbbbbbbb1",
		".1111111111.",
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
	chest                                 sprite.Frame
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
	action           sprite.Animation
	round            time.Duration
	actionRound      time.Duration
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
	cinderTrail sheetActor
	bellToll    sheetActor
	thorn       sheetActor
	bone        sheetActor
	cache       sheetActor
	// chest is row 0's closed chest glowing and row 1's opening; the rest are the evolved weapons' looks.
	chest    sheetActor
	starfall sheetActor
	sunRing  sheetActor
	wildfire sheetActor
	dawnBell sheetActor
}

// sheetSpec describes one uniform-grid PNG sprite sheet under assets/sprites/.
// When actionFrameTime > 0 on a 2-row sheet, row 0 is the normal movement clip and row 1 is the attack/telegraph clip;
// otherwise row 0 is idle and row 1 is walk (as on keeper.png).
type sheetSpec struct {
	name             string
	file             string
	rows             int
	cols             int
	idleFrameTime    time.Duration
	moveFrameTime    time.Duration
	actionFrameTime  time.Duration
	worldHeight      float32
	defaultFacesLeft bool
	target           func(*sheetFrames) *sheetActor
}

// sheetSpecs lists every stylized 2D sprite sheet the loader recognizes, falling back cleanly when a sheet is absent.
var sheetSpecs = []sheetSpec{
	{
		name: "keeper", file: "assets/sprites/keeper.png",
		rows: 2, cols: 4, idleFrameTime: 180 * time.Millisecond, moveFrameTime: 110 * time.Millisecond,
		worldHeight: 28,
		target:      func(s *sheetFrames) *sheetActor { return &s.keeper },
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
		rows: 2, cols: 4, moveFrameTime: 140 * time.Millisecond, actionFrameTime: 115 * time.Millisecond,
		worldHeight: 38,
		target:      func(s *sheetFrames) *sheetActor { return &s.gloomwarden },
	},
	{
		name: "hollow_king", file: "assets/sprites/hollow_king.png",
		rows: 2, cols: 4, moveFrameTime: 150 * time.Millisecond, actionFrameTime: 120 * time.Millisecond,
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
		name: "lantern_halo", file: "assets/sprites/lantern_halo.png",
		rows: 1, cols: 4, moveFrameTime: 100 * time.Millisecond,
		worldHeight: 16,
		target:      func(s *sheetFrames) *sheetActor { return &s.halo },
	},
	{
		name: "cinder_trail", file: "assets/sprites/cinder_trail.png",
		rows: 1, cols: 4, moveFrameTime: 110 * time.Millisecond,
		worldHeight: 32,
		target:      func(s *sheetFrames) *sheetActor { return &s.cinderTrail },
	},
	{
		name: "bell_toll", file: "assets/sprites/bell_toll.png",
		rows: 1, cols: 4, moveFrameTime: 90 * time.Millisecond,
		worldHeight: 64,
		target:      func(s *sheetFrames) *sheetActor { return &s.bellToll },
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
	{
		name: "chest", file: "assets/sprites/chest.png",
		rows: 2, cols: 4, moveFrameTime: 140 * time.Millisecond, actionFrameTime: 110 * time.Millisecond,
		worldHeight: 20,
		target:      func(s *sheetFrames) *sheetActor { return &s.chest },
	},
	{
		name: "starfall_volley", file: "assets/sprites/starfall_volley.png",
		rows: 1, cols: 4, moveFrameTime: 70 * time.Millisecond,
		worldHeight: 9,
		target:      func(s *sheetFrames) *sheetActor { return &s.starfall },
	},
	{
		name: "sun_ring", file: "assets/sprites/sun_ring.png",
		rows: 1, cols: 4, moveFrameTime: 100 * time.Millisecond,
		worldHeight: 64,
		target:      func(s *sheetFrames) *sheetActor { return &s.sunRing },
	},
	{
		name: "wildfire_path", file: "assets/sprites/wildfire_path.png",
		rows: 1, cols: 4, moveFrameTime: 110 * time.Millisecond,
		worldHeight: 64,
		target:      func(s *sheetFrames) *sheetActor { return &s.wildfire },
	},
	{
		name: "dawn_bell", file: "assets/sprites/dawn_bell.png",
		rows: 1, cols: 4, moveFrameTime: 90 * time.Millisecond,
		worldHeight: 64,
		target:      func(s *sheetFrames) *sheetActor { return &s.dawnBell },
	},
}

// envArt holds the decoded hand-painted arena tiles, forest props and border canopy from assets/environment/.
type envArt struct {
	soil         *image.RGBA
	moss         *image.RGBA
	grassTuft    *image.RGBA
	fern         *image.RGBA
	mossStone    *image.RGBA
	mushrooms    *image.RGBA
	stump        *image.RGBA
	fallenBranch *image.RGBA
	treeCanopy   *image.RGBA
}

// loaded reports whether the required seamless ground tiles decoded cleanly.
func (e envArt) loaded() bool {
	return e.soil != nil && e.moss != nil
}

// art holds the procedural fallback atlas, the packed stylized sprite-sheet atlas, embedded environment tiles/props and upgrade card icons.
type art struct {
	atlas      *sprite.Atlas
	frames     frames
	pixels     map[string]*pixels
	sheetAtlas *sprite.Atlas
	sheets     sheetFrames
	env        envArt
	icons      map[string]core.ImageSource
}

// newArt builds the procedural fallback atlas and loads all embedded sprite sheets, environment art and upgrade card icons.
func newArt() *art {
	a := newArtFromAllFS(embeddedSprites, embeddedIcons, embeddedEnvironment)
	return a
}

// newArtFromFS builds the procedural fallback atlas and loads any available sprite sheets and icons from spriteFS and iconFS,
// using procedural ground fallback.
func newArtFromFS(spriteFS, iconFS fs.FS) *art {
	return newArtFromAllFS(spriteFS, iconFS, nil)
}

// newArtFromAllFS builds the procedural fallback atlas and loads any available sprite sheets, icons and environment assets.
// Missing or unreadable files are skipped so callers always get a valid fallback.
func newArtFromAllFS(spriteFS, iconFS, envFS fs.FS) *art {
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
	a.loadEnvironment(envFS)
	return a
}

// hasPaintedGround reports whether the hand-painted forest floor tiles are loaded.
func (a *art) hasPaintedGround() bool {
	return a != nil && a.env.loaded()
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
		sheetAtlasWidth = 768
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
		if spec.rows >= 2 && spec.actionFrameTime > 0 {
			actor.move = buildClip(gridFrames[si][0], spec.moveFrameTime)
			actor.idle = actor.move
			actor.action = buildClip(gridFrames[si][1], spec.actionFrameTime)
			actor.round = spec.moveFrameTime * time.Duration(spec.cols)
			actor.actionRound = spec.actionFrameTime * time.Duration(spec.cols)
			atlas.Animations[spec.name+":move"] = actor.move
			atlas.Animations[spec.name+":action"] = actor.action
		} else if spec.rows >= 2 {
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

// loadEnvironment decodes the hand-painted forest floor tiles, props and border canopy from assets/environment/.
// Missing or corrupt files are skipped cleanly so newGround falls back to procedural rendering whenever needed.
func (a *art) loadEnvironment(fsys fs.FS) {
	if fsys == nil {
		return
	}
	load := func(name string) *image.RGBA {
		raw, err := fs.ReadFile(fsys, "assets/environment/"+name)
		if err != nil {
			return nil
		}
		rgba, err := decodePNGToRGBA(raw)
		if err != nil {
			return nil
		}
		b := rgba.Bounds()
		if b.Dx() <= 0 || b.Dy() <= 0 {
			return nil
		}
		return rgba
	}
	a.env = envArt{
		soil:         load("ground_soil.png"),
		moss:         load("ground_moss.png"),
		grassTuft:    load("grass_tuft.png"),
		fern:         load("fern.png"),
		mossStone:    load("moss_stone.png"),
		mushrooms:    load("mushrooms.png"),
		stump:        load("stump.png"),
		fallenBranch: load("fallen_branch.png"),
		treeCanopy:   load("tree_canopy.png"),
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
		chest:       get("chest"),
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

// The ground's look: the size of a soil block, how many pieces of decor the run's cosmetic stream scatters over the arena,
// and the moonlit twilight soil shades so the forest floor reads clearly even outside the lantern's warm core.
const (
	soilBlock  = 4
	decorSpots = 1100
)

var (
	groundSoilBase      = core.RGB(0x283246)
	groundSoilMoss      = core.RGB(0x2f483a)
	groundSoilShadow    = core.RGB(0x202838)
	groundSoilHighlight = core.RGB(0x344159)
)

// newGround paints the arena's ground once per run from the run's seed. When the hand-painted environment tiles
// are embedded, it blends ground_soil and ground_moss seamlessly, scatters proportionally scaled forest props,
// and frames the four arena edges with overlapping tree_canopy. Otherwise it falls back to procedural moonlit soil.
func (a *art) newGround(seed uint64, width, height int) *image.RGBA {
	if a.hasPaintedGround() {
		return a.newPaintedGround(seed, width, height)
	}
	return a.newProceduralGround(seed, width, height)
}

// newProceduralGround paints mottled moonlit soil and procedural pixel decor when hand-painted tiles are absent.
func (a *art) newProceduralGround(seed uint64, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y += soilBlock {
		for x := 0; x < width; x += soilBlock {
			soil := groundSoilBase
			switch mottle(seed, x/soilBlock, y/soilBlock) & 31 {
			case 0, 1:
				soil = groundSoilMoss
			case 2, 3:
				soil = groundSoilShadow
			case 4:
				soil = groundSoilHighlight
			}
			fillInto(img, x, y, soilBlock, soilBlock, soil)
		}
	}

	decor := a.frames.decor()
	s := random.NewStreams(seed).Stream("ground")
	for range decorSpots {
		p := a.pixels[decor[decorChoice(s, len(decor))].Name]
		x, y := s.Intn(width)-p.w/2, s.Intn(height)-p.h/2
		p.drawInto(img, x, y)
	}
	return img
}

// newPaintedGround tiles and smoothly blends ground_soil.png and ground_moss.png, scatters proportionally
// scaled transparent forest props, and rings the outer arena perimeter with tree_canopy.png.
func (a *art) newPaintedGround(seed uint64, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if width <= 0 || height <= 0 {
		return img
	}

	soil, moss := a.env.soil, a.env.moss
	soilW, soilH := soil.Bounds().Dx(), soil.Bounds().Dy()
	mossW, mossH := moss.Bounds().Dx(), moss.Bounds().Dy()

	s := random.NewStreams(seed).Stream("ground")
	soilOffX, soilOffY := s.Intn(soilW), s.Intn(soilH)
	mossOffX, mossOffY := s.Intn(mossW), s.Intn(mossH)

	// Precompute two coarse value-noise grids for organic, seam-free moss glades.
	const (
		coarseCell = 160
		detailCell = 64
	)
	coarse := newNoiseGrid(seed^0x51a7c3d1, width, height, coarseCell)
	detail := newNoiseGrid(seed^0x9e3779b9, width, height, detailCell)

	for y := 0; y < height; y++ {
		sy := (y + soilOffY) % soilH
		my := (y + mossOffY) % mossH
		soilRow := sy * soil.Stride
		mossRow := my * moss.Stride
		dstRow := y * img.Stride
		for x := 0; x < width; x++ {
			n := 0.72*coarse.sample(x, y) + 0.28*detail.sample(x, y)
			w := smoothstep(0.36, 0.66, n)
			mw := uint32(w*256 + 0.5)
			sw := 256 - mw

			sx := (x + soilOffX) % soilW
			mx := (x + mossOffX) % mossW
			si := soilRow + sx*4
			mi := mossRow + mx*4
			di := dstRow + x*4

			img.Pix[di+0] = uint8((uint32(soil.Pix[si+0])*sw + uint32(moss.Pix[mi+0])*mw) >> 8)
			img.Pix[di+1] = uint8((uint32(soil.Pix[si+1])*sw + uint32(moss.Pix[mi+1])*mw) >> 8)
			img.Pix[di+2] = uint8((uint32(soil.Pix[si+2])*sw + uint32(moss.Pix[mi+2])*mw) >> 8)
			img.Pix[di+3] = 255
		}
	}

	// Scatter proportionally scaled forest props (keeper is ~28 logical units tall).
	type envProp struct {
		img         *image.RGBA
		minH, maxH  float64
		weight      float64
		sizeByWidth bool
	}
	candidates := []envProp{
		{img: a.env.grassTuft, minH: 16, maxH: 22, weight: 5.5},
		{img: a.env.fern, minH: 18, maxH: 24, weight: 4.0},
		{img: a.env.mossStone, minH: 16, maxH: 22, weight: 2.0},
		{img: a.env.mushrooms, minH: 16, maxH: 20, weight: 1.4},
		{img: a.env.fallenBranch, minH: 28, maxH: 38, weight: 1.2, sizeByWidth: true},
		{img: a.env.stump, minH: 28, maxH: 36, weight: 0.9},
	}
	props := candidates[:0]
	weights := make([]float64, 0, len(candidates))
	for _, p := range candidates {
		if p.img != nil {
			props = append(props, p)
			weights = append(weights, p.weight)
		}
	}
	if len(props) > 0 {
		count := max(24, (width*height)/8000)
		margin := min(24, min(width, height)/8)
		spanX := max(1, width-2*margin)
		spanY := max(1, height-2*margin)
		for range count {
			p := props[s.WeightedIndex(weights)]
			sb := p.img.Bounds()
			target := s.Range(p.minH, p.maxH)
			var dw, dh int
			if p.sizeByWidth {
				dw = max(1, int(math.Round(target)))
				dh = max(1, int(math.Round(target*float64(sb.Dy())/float64(sb.Dx()))))
			} else {
				dh = max(1, int(math.Round(target)))
				dw = max(1, int(math.Round(target*float64(sb.Dx())/float64(sb.Dy()))))
			}
			cx := margin + s.Intn(spanX)
			cy := margin + s.Intn(spanY)
			blitScaledOver(img, p.img, cx, cy, dw, dh, s.Intn(2) == 1)
		}
	}

	// Frame all four outer arena edges with slightly overlapping tree canopies (purely decorative).
	if canopy := a.env.treeCanopy; canopy != nil {
		const (
			canopyStep   = 38
			canopyMin    = 56.0
			canopyMax    = 72.0
			edgeInsetMax = 16.0
		)
		placeCanopy := func(cx, cy float64) {
			size := max(1, int(math.Round(s.Range(canopyMin, canopyMax))))
			blitScaledOver(img, canopy, int(math.Round(cx)), int(math.Round(cy)), size, size, s.Intn(2) == 1)
		}
		for x := 0; x <= width; x += canopyStep {
			placeCanopy(float64(x)+s.Range(-10, 10), s.Range(-8, edgeInsetMax))
			placeCanopy(float64(x)+s.Range(-10, 10), float64(height)-s.Range(-8, edgeInsetMax))
		}
		for y := canopyStep; y < height; y += canopyStep {
			placeCanopy(s.Range(-8, edgeInsetMax), float64(y)+s.Range(-10, 10))
			placeCanopy(float64(width)-s.Range(-8, edgeInsetMax), float64(y)+s.Range(-10, 10))
		}
	}

	return img
}

// noiseGrid is a coarse 2D deterministic value-noise table sampled with smoothstep bilinear interpolation.
type noiseGrid struct {
	cols int
	cell int
	vals []float64
}

func newNoiseGrid(seed uint64, width, height, cell int) noiseGrid {
	cols := width/cell + 2
	rows := height/cell + 2
	vals := make([]float64, cols*rows)
	for gy := 0; gy < rows; gy++ {
		for gx := 0; gx < cols; gx++ {
			vals[gy*cols+gx] = float64(mottle(seed, gx, gy)&0xffff) / 65535.0
		}
	}
	return noiseGrid{cols: cols, cell: cell, vals: vals}
}

func (g noiseGrid) sample(x, y int) float64 {
	gx, gy := x/g.cell, y/g.cell
	tx := float64(x-gx*g.cell) / float64(g.cell)
	ty := float64(y-gy*g.cell) / float64(g.cell)
	tx = tx * tx * (3 - 2*tx)
	ty = ty * ty * (3 - 2*ty)

	i00 := gy*g.cols + gx
	v00 := g.vals[i00]
	v10 := g.vals[i00+1]
	v01 := g.vals[i00+g.cols]
	v11 := g.vals[i00+g.cols+1]
	top := v00 + (v10-v00)*tx
	bot := v01 + (v11-v01)*tx
	return top + (bot-top)*ty
}

// smoothstep maps v smoothly from 0 at edge0 to 1 at edge1.
func smoothstep(edge0, edge1, v float64) float64 {
	t := (v - edge0) / (edge1 - edge0)
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// blitScaledOver draws premultiplied RGBA src scaled to dstW×dstH centered at (cx, cy) onto dst with
// bilinear filtering and Porter-Duff Over alpha compositing.
func blitScaledOver(dst, src *image.RGBA, cx, cy, dstW, dstH int, flipX bool) {
	if dstW <= 0 || dstH <= 0 {
		return
	}
	sb := src.Bounds()
	srcW, srcH := sb.Dx(), sb.Dy()
	if srcW <= 0 || srcH <= 0 {
		return
	}
	db := dst.Bounds()
	x0 := cx - dstW/2
	y0 := cy - dstH/2
	startX := max(x0, db.Min.X)
	endX := min(x0+dstW, db.Max.X)
	startY := max(y0, db.Min.Y)
	endY := min(y0+dstH, db.Max.Y)
	if startX >= endX || startY >= endY {
		return
	}

	scaleX := float64(srcW) / float64(dstW)
	scaleY := float64(srcH) / float64(dstH)
	maxSX := srcW - 1
	maxSY := srcH - 1

	for dy := startY; dy < endY; dy++ {
		syF := (float64(dy-y0)+0.5)*scaleY - 0.5
		if syF < 0 {
			syF = 0
		} else if syF > float64(maxSY) {
			syF = float64(maxSY)
		}
		sy0 := int(syF)
		sy1 := min(sy0+1, maxSY)
		fy := syF - float64(sy0)
		wy0 := uint32((1-fy)*256 + 0.5)
		wy1 := 256 - wy0
		row0 := sy0 * src.Stride
		row1 := sy1 * src.Stride
		dstRow := dy * dst.Stride

		for dx := startX; dx < endX; dx++ {
			relX := float64(dx-x0) + 0.5
			if flipX {
				relX = float64(dstW) - relX
			}
			sxF := relX*scaleX - 0.5
			if sxF < 0 {
				sxF = 0
			} else if sxF > float64(maxSX) {
				sxF = float64(maxSX)
			}
			sx0 := int(sxF)
			sx1 := min(sx0+1, maxSX)
			fx := sxF - float64(sx0)
			wx0 := uint32((1-fx)*256 + 0.5)
			wx1 := 256 - wx0

			w00 := wx0 * wy0
			w10 := wx1 * wy0
			w01 := wx0 * wy1
			w11 := wx1 * wy1

			p00 := row0 + sx0*4
			p10 := row0 + sx1*4
			p01 := row1 + sx0*4
			p11 := row1 + sx1*4

			sa := (uint32(src.Pix[p00+3])*w00 + uint32(src.Pix[p10+3])*w10 + uint32(src.Pix[p01+3])*w01 + uint32(src.Pix[p11+3])*w11) >> 16
			if sa == 0 {
				continue
			}
			sr := (uint32(src.Pix[p00+0])*w00 + uint32(src.Pix[p10+0])*w10 + uint32(src.Pix[p01+0])*w01 + uint32(src.Pix[p11+0])*w11) >> 16
			sg := (uint32(src.Pix[p00+1])*w00 + uint32(src.Pix[p10+1])*w10 + uint32(src.Pix[p01+1])*w01 + uint32(src.Pix[p11+1])*w11) >> 16
			sbC := (uint32(src.Pix[p00+2])*w00 + uint32(src.Pix[p10+2])*w10 + uint32(src.Pix[p01+2])*w01 + uint32(src.Pix[p11+2])*w11) >> 16

			di := dstRow + dx*4
			if sa >= 255 {
				dst.Pix[di+0] = uint8(sr)
				dst.Pix[di+1] = uint8(sg)
				dst.Pix[di+2] = uint8(sbC)
				dst.Pix[di+3] = 255
				continue
			}
			invA := 255 - sa
			dst.Pix[di+0] = uint8(min(255, sr+(uint32(dst.Pix[di+0])*invA+127)/255))
			dst.Pix[di+1] = uint8(min(255, sg+(uint32(dst.Pix[di+1])*invA+127)/255))
			dst.Pix[di+2] = uint8(min(255, sbC+(uint32(dst.Pix[di+2])*invA+127)/255))
			dst.Pix[di+3] = uint8(min(255, sa+(uint32(dst.Pix[di+3])*invA+127)/255))
		}
	}
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

// fogScale is how many world units one pixel of the fog of war's mask spans; the mask is soft, so a coarse one scaled up smoothly reads the same.
const fogScale = 4

// fogRGB is the near-black of the fog of war, a touch of night blue so it reads as cloud rather than a hole.
const fogRGB = 0x05070d

var fogColor = core.RGB(fogRGB)

// newFogMask paints the fog of war over a width×height arena: clear inside the field fog world units from every edge,
// thickening through a ragged, cloudy rim into solid darkness at the arena's border. It derives from seed alone.
func newFogMask(seed uint64, width, height int, fog float64) *image.RGBA {
	mw, mh := (width+fogScale-1)/fogScale, (height+fogScale-1)/fogScale
	img := image.NewRGBA(image.Rect(0, 0, mw, mh))
	if mw <= 0 || mh <= 0 || fog <= 0 {
		return img
	}
	edge := newNoiseGrid(seed^0x6f9c2b17, width, height, 96)
	cloud := newNoiseGrid(seed^0x2d4e8a53, width, height, 40)
	fr, fg, fb, _ := fogColor.Components()
	w, h := float64(width), float64(height)
	// The rim starts at the field's edge and turns solid well before the arena's, so the border is never seen.
	depth := fog * 0.45
	ragged := fog * 0.15
	for my := range mh {
		y := min(my*fogScale+fogScale/2, height-1)
		for mx := range mw {
			x := min(mx*fogScale+fogScale/2, width-1)
			// How far the point lies past the field's edge, toward the arena's border.
			out := fog - min(float64(x), w-float64(x), float64(y), h-float64(y))
			if out <= -ragged {
				continue
			}
			t := smoothstep(0, depth, out+(edge.sample(x, y)-0.5)*2*ragged)
			if t <= 0 {
				continue
			}
			// Wisps of thinner cloud drift through the rim; the solid fog beyond it stays solid.
			a := t * (0.82 + 0.18*cloud.sample(x, y))
			a = a + (1-a)*smoothstep(0.7, 1, t)
			i := img.PixOffset(mx, my)
			img.Pix[i+0] = uint8(fr*a*255 + 0.5)
			img.Pix[i+1] = uint8(fg*a*255 + 0.5)
			img.Pix[i+2] = uint8(fb*a*255 + 0.5)
			img.Pix[i+3] = uint8(a*255 + 0.5)
		}
	}
	return img
}
