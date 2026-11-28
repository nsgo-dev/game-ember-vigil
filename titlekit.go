package main

import (
	"image"
	imagedraw "image/draw"
	"io/fs"
	"sort"

	"github.com/nsgo-dev/nsgo/core"
)

// titleAtlasW is the width of the atlas the title's small pieces share, the width of its widest piece, the wordmark.
const (
	titleAtlasW   = 1024
	titleAtlasPad = 2
)

// piece is one part of the title kit as the canvas draws it: the rectangle sx, sy, sw, sh of a packed image, and x, y, w, h, where it lies in the kit's 1920×1080 frame for a layer or its own size otherwise, in the kit's pixels.
type piece struct {
	img            core.ImageSource
	sx, sy, sw, sh float32
	x, y, w, h     float32
}

// titleArt is the decoded title kit, packed so a frame of the title draws five images: the background, the mist, the keeper, the canopy and one atlas of everything small. Apple, Windows and Linux keep only eight pixel images a canvas drew, so the kit as painted, eleven images of which four are full frames, made them copy every layer again on every frame.
//
// The cover animates only when every layer, the glow and the embers loaded; otherwise the title draws a still procedural cover.
type titleArt struct {
	pieces map[string][]piece
	embers []piece
	cover  bool
}

// newTitleArt decodes every PNG under assets/title/ in fsys and packs them: the mist at half size, as it is soft anyway, the keeper and the canopy cut to their painted parts, and the rest in one atlas.
func newTitleArt(fsys fs.FS) *titleArt {
	raw := decodeKit(fsys, "assets/title")
	a := &titleArt{pieces: map[string][]piece{}}
	rgba := func(name string) *image.RGBA {
		return raw[name]
	}
	if img := rgba(titleBG); img != nil {
		a.pieces[titleBG] = []piece{wholePiece(img, image.Rectangle{Max: img.Bounds().Max})}
	}
	if img := rgba(titleMist); img != nil {
		half := halveRGBA(img)
		p := wholePiece(half, image.Rectangle{Max: img.Bounds().Max})
		a.pieces[titleMist] = []piece{p}
	}
	if img := rgba(titleKeeper); img != nil {
		b := alphaBounds(img, img.Bounds())
		a.pieces[titleKeeper] = []piece{wholePiece(cropRGBA(img, b), b)}
	}
	if img := rgba(titleCanopy); img != nil {
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		a.pieces[titleCanopy] = packPieces(img, []image.Rectangle{
			alphaBounds(img, image.Rect(0, 0, w/2, h)),
			alphaBounds(img, image.Rect(w/2, 0, w, h)),
		})
	}

	var names []string
	for _, name := range append([]string{titleGlow, titleLogoEN, titleLogoZhCN, titleButton, titleButtonFocus, titleButtonPress}, titleEmberNames[:]...) {
		if rgba(name) != nil {
			names = append(names, name)
		}
	}
	for name, p := range packAtlas(names, rgba) {
		a.pieces[name] = []piece{p}
	}

	a.cover = true
	for _, name := range []string{titleBG, titleMist, titleGlow, titleKeeper, titleCanopy} {
		a.cover = a.cover && len(a.pieces[name]) > 0
	}
	for _, name := range titleEmberNames {
		p, ok := a.piece(name)
		a.embers = append(a.embers, p)
		a.cover = a.cover && ok
	}
	return a
}

// piece returns the named piece, or false when the kit lacks it.
func (a *titleArt) piece(name string) (piece, bool) {
	if a == nil || len(a.pieces[name]) == 0 {
		return piece{}, false
	}
	return a.pieces[name][0], true
}

func (a *titleArt) animated() bool { return a != nil && a.cover }

// draw draws p into x, y, w, h.
func (p piece) draw(d *core.Drawing, x, y, w, h float32) {
	d.DrawImageFrom(p.img, p.sx, p.sy, p.sw, p.sh, x, y, w, h)
}

// imageSource hands img to the canvas as pixels measured in their own size.
func imageSource(img *image.RGBA) core.ImageSource {
	b := img.Bounds()
	return core.ImageSource{Pixels: img, Version: 1, Size: core.Size{Width: core.Points(b.Dx()), Height: core.Points(b.Dy())}}
}

// wholePiece is all of img, lying over at in the kit's frame.
func wholePiece(img *image.RGBA, at image.Rectangle) piece {
	b := img.Bounds()
	return piece{
		img: imageSource(img),
		sw:  float32(b.Dx()), sh: float32(b.Dy()),
		x: float32(at.Min.X), y: float32(at.Min.Y), w: float32(at.Dx()), h: float32(at.Dy()),
	}
}

// packPieces copies the parts rs of img side by side into one image, each piece lying where its part did.
func packPieces(img *image.RGBA, rs []image.Rectangle) []piece {
	w, h := 0, 0
	for _, r := range rs {
		w += r.Dx() + titleAtlasPad
		h = max(h, r.Dy())
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	src := imageSource(dst)
	out := make([]piece, 0, len(rs))
	x := 0
	for _, r := range rs {
		imagedraw.Draw(dst, image.Rect(x, 0, x+r.Dx(), r.Dy()), img, r.Min, imagedraw.Src)
		out = append(out, piece{
			img: src,
			sx:  float32(x), sw: float32(r.Dx()), sh: float32(r.Dy()),
			x: float32(r.Min.X), y: float32(r.Min.Y), w: float32(r.Dx()), h: float32(r.Dy()),
		})
		x += r.Dx() + titleAtlasPad
	}
	return out
}

// packAtlas packs the named images in rows, tallest first, into one atlas, each piece at its own size.
func packAtlas(names []string, rgba func(string) *image.RGBA) map[string]piece {
	out := map[string]piece{}
	if len(names) == 0 {
		return out
	}
	sort.SliceStable(names, func(i, j int) bool { return rgba(names[i]).Bounds().Dy() > rgba(names[j]).Bounds().Dy() })
	places := map[string]image.Point{}
	x, y, rowH, w := 0, 0, 0, titleAtlasW
	for _, name := range names {
		w = max(w, rgba(name).Bounds().Dx())
	}
	for _, name := range names {
		b := rgba(name).Bounds()
		if x > 0 && x+b.Dx() > w {
			x, y, rowH = 0, y+rowH+titleAtlasPad, 0
		}
		places[name] = image.Pt(x, y)
		x += b.Dx() + titleAtlasPad
		rowH = max(rowH, b.Dy())
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, y+rowH))
	src := imageSource(dst)
	for _, name := range names {
		img, at := rgba(name), places[name]
		b := img.Bounds()
		imagedraw.Draw(dst, image.Rectangle{Min: at, Max: at.Add(b.Size())}, img, b.Min, imagedraw.Src)
		out[name] = piece{
			img: src,
			sx:  float32(at.X), sy: float32(at.Y), sw: float32(b.Dx()), sh: float32(b.Dy()),
			w: float32(b.Dx()), h: float32(b.Dy()),
		}
	}
	return out
}

// alphaBounds is the smallest rectangle within r that holds every pixel of img that is not fully transparent.
func alphaBounds(img *image.RGBA, r image.Rectangle) image.Rectangle {
	out := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[img.PixOffset(r.Min.X, y):]
		for x := 0; x < r.Dx(); x++ {
			if row[x*4+3] != 0 {
				out = out.Union(image.Rect(r.Min.X+x, y, r.Min.X+x+1, y+1))
			}
		}
	}
	if out.Empty() {
		return image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Min.Y+1)
	}
	return out
}

// cropRGBA copies the part r of img.
func cropRGBA(img *image.RGBA, r image.Rectangle) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	imagedraw.Draw(dst, dst.Bounds(), img, r.Min, imagedraw.Src)
	return dst
}

// halveRGBA shrinks img to half its size, averaging each square of four pixels.
func halveRGBA(img *image.RGBA) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx()/2, b.Dy()/2
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			o := dst.PixOffset(x, y)
			a, c := img.PixOffset(b.Min.X+2*x, b.Min.Y+2*y), img.PixOffset(b.Min.X+2*x, b.Min.Y+2*y+1)
			for i := range 4 {
				sum := int(img.Pix[a+i]) + int(img.Pix[a+4+i]) + int(img.Pix[c+i]) + int(img.Pix[c+4+i])
				dst.Pix[o+i] = uint8((sum + 2) / 4)
			}
		}
	}
	return dst
}
