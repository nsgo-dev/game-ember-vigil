package main

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/gamepad"
	"github.com/nsgo-dev/nsgo/nsgotest"
	evstrings "me.oneo/embervigil/strings"
)

// fullShop is every shop item at its last level.
func fullShop() map[string]int {
	shop := &mustGameData().progression.Shop
	levels := map[string]int{}
	for _, it := range shop.Items {
		levels[it.ID] = shop.maxLevel()
	}
	return levels
}

// TestShopFollowsTheDesign checks the shop of docs/meta-progression.md: six items of five levels costing 50 to 250, about 4500 in all, with the bonuses at max.
func TestShopFollowsTheDesign(t *testing.T) {
	shop := &mustGameData().progression.Shop
	if !slices.Equal(shop.Costs, []int{50, 100, 150, 200, 250}) {
		t.Fatalf("the shop costs %v", shop.Costs)
	}
	total := 0
	for _, c := range shop.Costs {
		total += c * len(shop.Items)
	}
	if len(shop.Items) != 6 || total != 4500 {
		t.Fatalf("the shop has %d items costing %d in all, want 6 and 4500", len(shop.Items), total)
	}
	want := map[string]string{"banked-fire": "+15%", "lamp-oil": "-10%", "hearth": "+50", "light-foot": "+15", "lure": "+25", "redraw": "+5"}
	for i := range shop.Items {
		it := &shop.Items[i]
		if got := shopBonus(it, shop.maxLevel()); got != want[it.ID] {
			t.Errorf("%s at max gives %s, want %s", it.ID, got, want[it.ID])
		}
	}
}

// TestShopLevelsFoldIntoTheRunAndHalveAtTier4 checks that bought levels raise the run's derived stats and redraws, start the keeper at the higher health, and count half from Long Night 4 on.
func TestShopLevelsFoldIntoTheRunAndHalveAtTier4(t *testing.T) {
	gain := func(tier int) (stats, int, float64) {
		base := NewRun(3, tier)
		r := NewRun(3, tier)
		r.SetShop(fullShop())
		a, b := base.stats, r.stats
		return stats{
			damageMul:    b.damageMul - a.damageMul,
			cooldownMul:  a.cooldownMul - b.cooldownMul,
			moveSpeed:    b.moveSpeed - a.moveSpeed,
			maxHealth:    b.maxHealth - a.maxHealth,
			pickupRadius: b.pickupRadius - a.pickupRadius,
		}, r.Snapshot().Redraws, r.keeper.health - b.maxHealth
	}
	full, redraws, missing := gain(1)
	want := stats{damageMul: 0.15, cooldownMul: 0.10, moveSpeed: 15, maxHealth: 50, pickupRadius: 25}
	near := func(a, b stats, k float64) bool {
		return math.Abs(a.damageMul-b.damageMul*k) < 1e-9 && math.Abs(a.cooldownMul-b.cooldownMul*k) < 1e-9 &&
			math.Abs(a.moveSpeed-b.moveSpeed*k) < 1e-9 && math.Abs(a.maxHealth-b.maxHealth*k) < 1e-9
	}
	if !near(full, want, 1) || math.Abs(full.pickupRadius-25) > 1e-9 || redraws != 5 || missing != 0 {
		t.Fatalf("a full shop at tier 1 adds %+v and %d redraws, %v health short; want %+v and 5", full, redraws, missing, want)
	}
	for _, tier := range []int{2, 3} {
		if got, n, _ := gain(tier); !near(got, want, 1) || n != 5 {
			t.Fatalf("a full shop at tier %d adds %+v and %d redraws, want them whole", tier, got, n)
		}
	}
	for _, tier := range []int{4, 5} {
		got, n, _ := gain(tier)
		// Tier 3's light also shortens the pull, so pickup gains half of 25 at 90%.
		if !near(got, want, 0.5) || math.Abs(got.pickupRadius-12.5*0.9) > 1e-9 || n != 2 {
			t.Fatalf("a full shop at tier %d adds %+v and %d redraws, want half and 2", tier, got, n)
		}
	}
	r := NewRun(3, 1)
	r.SetShop(map[string]int{"hearth": 99, "redraw": -2, "unknown": 3})
	if r.stats.maxHealth != NewRun(3, 1).stats.maxHealth+50 || r.redraws != 0 {
		t.Fatalf("levels out of range give %v max health and %d redraws", r.stats.maxHealth, r.redraws)
	}
}

// TestRedrawDealsNewCardsUntilSpent checks that a redraw replaces every offered card while the pool has others, and is refused once spent or with no level-up open.
func TestRedrawDealsNewCardsUntilSpent(t *testing.T) {
	r := quietRun(t)
	r.SetShop(map[string]int{"redraw": 2})
	if r.Redraw() {
		t.Fatal("a redraw with no level-up open was taken")
	}
	r.xp = r.xpNeeded()
	stepOnce(r, Controls{})
	if r.Snapshot().Phase != PhaseLevelUp {
		t.Fatal("no level-up opened")
	}
	ids := func() []string {
		var out []string
		for _, o := range r.Snapshot().Offers {
			out = append(out, o.ID)
		}
		return out
	}
	for left := 1; left >= 0; left-- {
		before := ids()
		if !r.Redraw() {
			t.Fatalf("a redraw with %d left was refused", left+1)
		}
		after := ids()
		if len(after) != offerCount {
			t.Fatalf("a redraw offers %v", after)
		}
		for _, id := range after {
			if slices.Contains(before, id) {
				t.Fatalf("a redraw of %v offers %v again", before, after)
			}
		}
		if got := r.Snapshot().Redraws; got != left {
			t.Fatalf("after a redraw %d are left, want %d", got, left)
		}
	}
	before := ids()
	if r.Redraw() || !slices.Equal(ids(), before) {
		t.Fatal("a spent redraw still dealt cards")
	}
	if !r.Choose(0) || r.Snapshot().Phase != PhasePlaying {
		t.Fatal("a redrawn card could not be chosen")
	}
}

// TestShopAndRedrawsReplayIdentically plays two runs with one seed, one full shop and one control script that redraws at every level-up, and compares their events and snapshots.
func TestShopAndRedrawsReplayIdentically(t *testing.T) {
	play := func() (*Run, []Event, int) {
		r := NewRun(4242, 4)
		r.SetShop(fullShop())
		var events []Event
		redrew := 0
		for step := range 3 * 60 * 60 {
			if r.Snapshot().Phase == PhaseLevelUp {
				if r.Redraw() {
					redrew++
				}
				r.Choose(0)
			}
			r.Step(stepTime, circleControls(step))
			events = append(events, r.Events()...)
		}
		return r, events, redrew
	}
	first, firstEvents, n := play()
	second, secondEvents, _ := play()
	if n != 2 {
		t.Fatalf("the run redrew %d times at tier 4, want its 2", n)
	}
	if !reflect.DeepEqual(firstEvents, secondEvents) || !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatal("the same seed, shop and controls ended in different runs")
	}
}

// TestBuyingSpendsRaisesAndStopsAtMax checks the buy rule: it spends the next level's cost, refuses what the balance cannot pay, an unknown item and a maxed one, and never changes the progression it started from.
func TestBuyingSpendsRaisesAndStopsAtMax(t *testing.T) {
	shop := &mustGameData().progression.Shop
	p := Progression{Embers: 800, Shop: map[string]int{"lure": 1}}
	next, ok := buyShopItem(p, shop, "hearth")
	if !ok || next.Embers != 750 || next.Shop["hearth"] != 1 || next.Shop["lure"] != 1 {
		t.Fatalf("buying Hearth gives %+v", next)
	}
	if p.Shop["hearth"] != 0 || p.Embers != 800 {
		t.Fatalf("buying changed the progression it started from: %+v", p)
	}
	for _, want := range []int{650, 500, 300, 50} {
		if next, ok = buyShopItem(next, shop, "hearth"); !ok || next.Embers != want {
			t.Fatalf("buying Hearth again leaves %d embers, want %d", next.Embers, want)
		}
	}
	if _, ok := next.shopCost(shop, "hearth"); ok || next.shopLevel(shop, "hearth") != 5 {
		t.Fatal("a maxed Hearth still has a next level")
	}
	next.Embers = 1000
	for _, id := range []string{"hearth", "unknown"} {
		if got, ok := buyShopItem(next, shop, id); ok || got.Embers != 1000 {
			t.Fatalf("buying %s gave %+v", id, got)
		}
	}
	if _, ok := buyShopItem(Progression{Embers: 99}, shop, "lure"); !ok {
		t.Fatal("a first level of 50 was refused with 99 embers")
	}
	if _, ok := buyShopItem(Progression{Embers: 49}, shop, "lure"); ok {
		t.Fatal("a first level of 50 was sold for 49 embers")
	}
}

// TestShopBuysByEveryInputAndTheNextRunTakesIt opens the shop from the title, buys by tap, keyboard and gamepad, refuses a maxed item, reads the shop after a restart, and starts a run with the bonuses and redraws bought, which tap, key and gamepad spend.
func TestShopBuysByEveryInputAndTheNextRunTakesIt(t *testing.T) {
	app, g := startEmberVigil(t, nsgotest.Storage(map[string]string{progressionStorageKey: `{"version":1,"embers":1000,"shop":{"hearth":4}}`}))
	press(t, app, "id:title-shop")
	if g.phase.Get() != phaseShop {
		t.Fatalf("Shop opened phase %v", g.phase.Get())
	}
	app.Find("id:shop-heading")
	en := evstrings.For(evstrings.EN)
	if got := app.Find("id:shop-balance").Props.Label; got != en.EmberBalance(1000) {
		t.Fatalf("the balance reads %q", got)
	}
	ops := app.Find("id:arena").Ops
	for _, name := range []string{hudShopRow, hudShopRowFocused, hudEmberEmblem} {
		if !drewHUDPiece(ops, g.hudArt, name) {
			t.Fatalf("the shop did not draw %s", name)
		}
	}
	if img, ok := g.art.cardIcon("banked-fire"); ok && !drewImage(ops, img) {
		t.Fatal("the shop did not draw Banked Fire's icon")
	}

	// A tap buys Hearth's last level; another tap on the maxed item changes nothing.
	press(t, app, "id:shop-item-hearth")
	if g.progress.Embers != 750 || g.progress.Shop["hearth"] != 5 || loadProgression(app.Storage()).Shop["hearth"] != 5 {
		t.Fatalf("buying Hearth by tap gave %+v", g.progress)
	}
	if got := app.Find("id:shop-item-hearth").Props.Hint; !strings.Contains(got, en.ShopMax) {
		t.Fatalf("a maxed Hearth reads %q", got)
	}
	press(t, app, "id:shop-item-hearth")
	if g.progress.Embers != 750 {
		t.Fatalf("a maxed Hearth sold again: %+v", g.progress)
	}

	// The keyboard moves from Hearth (row 2, left) down to Lure and buys it; the gamepad moves right to Redraw and buys it.
	tapKey := func(code string) {
		app.KeyCodeDown(code)
		app.Advance(2 * frame)
		app.KeyCodeUp(code)
		app.Advance(frame)
	}
	tapKey("KeyS")
	if g.menuFocus.Get() != 4 {
		t.Fatalf("down from Hearth focused %d, want Lure's 4", g.menuFocus.Get())
	}
	app.KeyDown("enter")
	app.Advance(2 * frame)
	app.KeyUp("enter")
	app.Advance(frame)
	if g.progress.Shop["lure"] != 1 || g.progress.Embers != 700 {
		t.Fatalf("buying Lure by keyboard gave %+v", g.progress)
	}
	app.ConnectGamepad(0, "Test Pad")
	tap := func(b gamepad.Button) {
		app.SetGamepadButton(0, b, 1)
		app.Advance(2 * frame)
		app.SetGamepadButton(0, b, 0)
		app.Advance(frame)
	}
	tap(gamepad.ButtonDPadRight)
	for range 2 {
		tap(gamepad.ButtonSouth)
	}
	if g.progress.Shop["redraw"] != 2 || g.progress.Embers != 550 {
		t.Fatalf("buying Redraw twice by gamepad gave %+v", g.progress)
	}
	tap(gamepad.ButtonDPadDown)
	if g.menuFocus.Get() != len(mustGameData().progression.Shop.Items) {
		t.Fatalf("down from the last row focused %d, want Back", g.menuFocus.Get())
	}
	tap(gamepad.ButtonSouth)
	if g.phase.Get() != phaseTitle {
		t.Fatalf("Back from the shop gave phase %v", g.phase.Get())
	}

	// After a restart the shop reads what was bought, and the next run starts with it.
	app2, g2 := startEmberVigil(t, nsgotest.Storage(storageSnapshot(app.Storage())))
	press(t, app2, "id:title-shop")
	if got := app2.Find("id:shop-item-redraw").Props.Hint; !strings.Contains(got, en.Level(2)+"/5") || !strings.Contains(got, en.ShopCost(150)) {
		t.Fatalf("Redraw after a restart reads %q", got)
	}
	press(t, app2, "id:shop-back")
	press(t, app2, "id:title-start")
	base := NewRun(1, 1)
	if s := g2.run.Snapshot(); s.MaxHealth != base.stats.maxHealth+50 || s.Health != s.MaxHealth || s.Redraws != 2 {
		t.Fatalf("the next run starts with %v of %v health and %d redraws", s.Health, s.MaxHealth, s.Redraws)
	}

	g2.run.xp = g2.run.xpNeeded()
	app2.Advance(2 * frame)
	offers := func() []OfferView { return slices.Clone(g2.run.Snapshot().Offers) }
	first := offers()
	press(t, app2, "id:level-up-redraw")
	if g2.run.Snapshot().Redraws != 1 || reflect.DeepEqual(offers(), first) {
		t.Fatal("tapping Redraw did not deal new cards")
	}
	app2.Advance(10 * frame)
	if !drewText(app2.Find("id:arena").Ops, en.Redraw(1)) {
		t.Fatal("the canvas did not draw the Redraw button with one left")
	}
	app2.KeyCodeDown("KeyR")
	app2.Advance(2 * frame)
	app2.KeyCodeUp("KeyR")
	app2.Advance(2 * frame)
	if g2.run.Snapshot().Redraws != 0 || app2.Exists("id:level-up-redraw") {
		t.Fatal("R did not spend the last redraw and hide the button")
	}
	chooseCard(t, app2, "id:level-up-card-0")

	// The gamepad's Back redraws too, given one more.
	g2.run.redraws = 1
	g2.run.xp = g2.run.xpNeeded()
	app2.Advance(2 * frame)
	app2.ConnectGamepad(0, "Test Pad")
	app2.SetGamepadButton(0, gamepad.ButtonBack, 1)
	app2.Advance(2 * frame)
	app2.SetGamepadButton(0, gamepad.ButtonBack, 0)
	app2.Advance(frame)
	if g2.run.Snapshot().Redraws != 0 || g2.run.Snapshot().Phase != PhaseLevelUp {
		t.Fatal("the gamepad's Back did not redraw")
	}
}

// TestShopLayoutFitsAndFocusMoves checks that the shop's panel, rows and Back fit the safe area without overlapping, and the focus walks the two columns and Back.
func TestShopLayoutFitsAndFocusMoves(t *testing.T) {
	n := len(mustGameData().progression.Shop.Items)
	for _, c := range []struct {
		size core.Size
		safe core.Edges
	}{
		{core.Size{Width: 1280, Height: 720}, core.Edges{}},
		{core.Size{Width: 960, Height: 540}, core.Edges{}},
		{core.Size{Width: 844, Height: 390}, core.Edges{Top: 12, Right: 44, Bottom: 21, Left: 44}},
	} {
		l := layoutShop(c.size, c.safe, n)
		area := overlayArea(c.size, c.safe)
		for _, r := range append(append([]rect{l.panel, l.back, l.note}, l.rows...), l.balance) {
			if r.x < area.x-0.5 || r.y < area.y-0.5 || r.x+r.w > area.x+area.w+0.5 || r.y+r.h > area.y+area.h+0.5 {
				t.Fatalf("%v: %+v leaves the safe area", c.size, r)
			}
		}
		for i, r := range l.rows {
			for _, o := range l.rows[:i] {
				if r.x < o.x+o.w && o.x < r.x+r.w && r.y < o.y+o.h && o.y < r.y+r.h {
					t.Fatalf("%v: row %d overlaps another", c.size, i)
				}
			}
		}
		cards := layoutCards(c.size, c.safe, offerCount, true)
		if r := cards.redraw; r.y < cards.cards[0].y+cards.cards[0].h || r.y+r.h > area.y+area.h+0.5 {
			t.Fatalf("%v: the Redraw button %+v sits on the cards or leaves the area", c.size, r)
		}
	}
	for _, m := range []struct{ focus, dx, dy, want int }{
		{0, 0, 1, 2}, {4, 0, 1, n}, {n, 0, 1, 0}, {n, 0, -1, 4}, {0, 0, -1, n}, {3, 0, -1, 1},
		{0, 1, 0, 1}, {1, 1, 0, 0}, {2, -1, 0, 3}, {n, 1, 0, n},
	} {
		if got := shopFocusMove(m.focus, n, m.dx, m.dy); got != m.want {
			t.Errorf("shopFocusMove(%d, %d, %d) = %d, want %d", m.focus, m.dx, m.dy, got, m.want)
		}
	}
}

// TestShopDrawsWithoutTheKit checks that the shop still draws its rows, words and costs when the HUD kit is absent.
func TestShopDrawsWithoutTheKit(t *testing.T) {
	size := core.Size{Width: 1280, Height: 720}
	shop := &mustGameData().progression.Shop
	for _, lang := range []evstrings.Lang{evstrings.EN, evstrings.ZhCN} {
		tr := evstrings.For(lang)
		d := &core.Drawing{}
		entries := shopEntries(tr, shop, Progression{Embers: 120, Shop: map[string]int{"hearth": 5}})
		drawShop(testPainter(d, size, newHUDArt(nil), settled(0), lang), layoutShop(size, core.Edges{}, len(entries)), entries, 120, 0)
		for _, want := range []string{tr.ShopTitle, tr.ShopItem("lure", "", "").Name, tr.ShopMax, "50", tr.ShopNow("+50"), tr.Back} {
			if !drewText(d.Ops, want) {
				t.Fatalf("%s: the shop without the kit did not write %q", lang, want)
			}
		}
	}
}
