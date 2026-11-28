package main

import (
	"math"
	"reflect"
	"sync"
	"testing"
)

// botResult summarizes one full automated run played by the balance bot.
type botResult struct {
	phase        Phase
	time         float64
	kills        int
	level        int
	minHealthPct float64
	bossSpawned  bool
	bossPhases   int
	eventCount   int
	seenEnemies  map[string]bool
	// bossTime is how long the Hollow King lived, and evolved the evolutions the run made.
	bossTime float64
	evolved  []string
}

// playBot runs a deterministic kiting and upgrade-picking bot until Victory, Defeat, or maxMinutes. It steers afresh every react steps and holds its last heading in between, so a larger react plays a slower player; evolve plays a player who has won once.
func playBot(seed uint64, tier, maxMinutes, react int, evolve bool) (*Run, botResult) {
	r := NewRun(seed, tier)
	if evolve {
		r.EnableEvolution()
	}
	bossAt := 0.0
	res := botResult{
		minHealthPct: 1,
		seenEnemies:  map[string]bool{},
	}
	maxSteps := maxMinutes * 60 * 60
	var controls Controls
	for step := range maxSteps {
		snap := r.Snapshot()
		if snap.MaxHealth > 0 {
			if pct := snap.Health / snap.MaxHealth; pct < res.minHealthPct {
				res.minHealthPct = pct
			}
		}
		for _, e := range snap.Enemies {
			res.seenEnemies[e.Kind] = true
		}
		switch snap.Phase {
		case PhaseVictory, PhaseDefeat:
			res.phase = snap.Phase
			res.time = snap.Time
			res.kills = snap.Kills
			res.level = snap.Level
			res.evolved = r.Evolutions()
			if res.bossSpawned {
				res.bossTime = snap.Time - bossAt
			}
			return r, res
		case PhaseLevelUp:
			r.Choose(botChoose(snap))
			continue
		}
		if step%react == 0 {
			controls = botControls(snap)
		}
		r.Step(stepTime, controls)
		for _, ev := range r.Events() {
			res.eventCount++
			switch ev.Kind {
			case EventBossSpawn:
				res.bossSpawned = true
				bossAt = r.Snapshot().Time
			case EventBossPhase:
				res.bossPhases++
			}
		}
	}
	snap := r.Snapshot()
	res.phase = snap.Phase
	res.time = snap.Time
	res.kills = snap.Kills
	res.level = snap.Level
	return r, res
}

// botControls computes a deterministic movement vector: dodge threats and charge lanes, kite crowds, and gather caches/embers.
func botControls(s Snapshot) Controls {
	var steer Vec2
	pos := s.Keeper

	// Repel from nearby enemies.
	for _, e := range s.Enemies {
		d := pos.Sub(e.Pos)
		dist := d.Len()
		avoid := e.Radius + 90
		if e.Boss {
			avoid = e.Radius + 130
		} else if e.Elite {
			avoid = e.Radius + 105
		}
		if dist < avoid && dist > 1e-6 {
			w := (avoid - dist) / avoid
			weight := 2.8 * w * w
			if e.Boss {
				weight *= 2.4
			}
			steer = steer.Add(d.Scale(weight / dist))
		}
	}

	// Step out of the shadow zones dead elites leave on a Long Night tier.
	for _, sh := range s.Shadows {
		d := pos.Sub(sh.Pos)
		dist := d.Len()
		avoid := sh.Radius + 24
		if dist < avoid && dist > 1e-6 {
			steer = steer.Add(d.Scale(3 * (avoid - dist) / avoid / dist))
		}
	}

	// Dodge incoming enemy shots.
	for _, sh := range s.EnemyShots {
		d := pos.Sub(sh.Pos)
		dist := d.Len()
		if dist < 95 && dist > 1e-6 {
			w := (95 - dist) / 95
			perp := Vec2{X: -d.Y, Y: d.X}.Scale(0.6 / dist)
			steer = steer.Add(d.Scale(3.6 * w * w / dist)).Add(perp.Scale(w))
		}
	}

	// Sidestep the Hollow King's charge telegraph lane.
	if s.Boss.Active && s.Boss.Telegraph == "charge" {
		rel := pos.Sub(s.Boss.Pos)
		dir := s.Boss.TelegraphDir
		along := rel.X*dir.X + rel.Y*dir.Y
		if along > -20 && along < s.Boss.TelegraphLength+40 {
			perp := Vec2{X: -dir.Y, Y: dir.X}
			side := rel.X*perp.X + rel.Y*perp.Y
			if side < 0 {
				perp = perp.Scale(-1)
			}
			steer = steer.Add(perp.Scale(5.0))
		}
	}

	// Seek elite healing caches when injured.
	if s.MaxHealth > 0 && s.Health/s.MaxHealth < 0.85 && len(s.Pickups) > 0 {
		bestIdx := -1
		bestDist := math.Inf(1)
		for i, p := range s.Pickups {
			if dist := p.Pos.Sub(pos).Len(); dist < bestDist {
				bestDist = dist
				bestIdx = i
			}
		}
		if bestIdx >= 0 && bestDist > 1 {
			steer = steer.Add(s.Pickups[bestIdx].Pos.Sub(pos).Normalized().Scale(1.6))
		}
	}

	// Open the nearest chest, as a player always would, unless a shadow zone covers it.
	if len(s.Chests) > 0 {
		bestIdx := -1
		bestDist := 400.0
	chests:
		for i, c := range s.Chests {
			// A chest in a shadow zone waits for the zone to fade.
			for _, sh := range s.Shadows {
				if c.Pos.Sub(sh.Pos).Len() < sh.Radius+24 {
					continue chests
				}
			}
			if dist := c.Pos.Sub(pos).Len(); dist < bestDist {
				bestDist = dist
				bestIdx = i
			}
		}
		if bestIdx >= 0 && bestDist > 1 {
			steer = steer.Add(s.Chests[bestIdx].Pos.Sub(pos).Normalized().Scale(1.2))
		}
	}

	// Seek the nearest safe ember to keep leveling up.
	if len(s.Embers) > 0 {
		bestIdx := -1
		bestDist := 220.0
		for i, em := range s.Embers {
			if dist := em.Pos.Sub(pos).Len(); dist < bestDist {
				bestDist = dist
				bestIdx = i
			}
		}
		if bestIdx >= 0 && bestDist > 1 {
			steer = steer.Add(s.Embers[bestIdx].Pos.Sub(pos).Normalized().Scale(0.65))
		}
	}

	// Stay away from arena corners/walls.
	margin := 180.0
	if pos.X < margin {
		steer.X += (margin - pos.X) / margin * 1.4
	} else if pos.X > s.Arena.X-margin {
		steer.X -= (pos.X - (s.Arena.X - margin)) / margin * 1.4
	}
	if pos.Y < margin {
		steer.Y += (margin - pos.Y) / margin * 1.4
	} else if pos.Y > s.Arena.Y-margin {
		steer.Y -= (pos.Y - (s.Arena.Y - margin)) / margin * 1.4
	}

	// Gentle orbit around the center when nothing is pressing.
	if steer.LenSq() < 0.04 {
		center := Vec2{X: s.Arena.X / 2, Y: s.Arena.Y / 2}
		toCenter := center.Sub(pos)
		if toCenter.Len() > 260 {
			steer = toCenter.Normalized()
		} else {
			steer = Vec2{X: math.Cos(s.Time * 0.5), Y: math.Sin(s.Time * 0.5)}
		}
	}

	return Controls{Move: steer.Normalized()}
}

// botChoose scores each offered card deterministically and returns the highest-priority index.
func botChoose(s Snapshot) int {
	best := 0
	bestScore := -1e9
	hpFrac := 1.0
	if s.MaxHealth > 0 {
		hpFrac = s.Health / s.MaxHealth
	}
	for i, o := range s.Offers {
		score := 0.0
		switch {
		case o.Heal:
			score = 10
			if hpFrac < 0.5 {
				score = 95
			}
		case o.Kind == offerWeapon:
			score = 70
			switch o.ID {
			case "ember-bolt":
				score += 18
			case "lantern-halo":
				score += 14
			case "bell-toll":
				score += 12
			case "cinder-trail":
				score += 8
			}
		case o.Kind == offerPassive:
			score = 45
			switch o.Stat {
			case statDamage:
				score += 20
			case statCooldown:
				score += 18
			case statArea:
				score += 14
			case statRegen:
				score += 16
			case statMaxHealth:
				score += 15
			case statMoveSpeed:
				score += 12
			case statXPGain:
				score += 10
			case statPickup:
				score += 8
			}
		}
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	return best
}

// TestTwelveMinuteBotBalanceAndDeterminism verifies that sharp movement wins most runs but not all and sees every enemy and boss phase, that evolution kills the Hollow King faster without winning more than that, that slow movement loses most of the same runs, that standing still loses, and that a run replays deterministically.
func TestTwelveMinuteBotBalanceAndDeterminism(t *testing.T) {
	const seeds = 10
	wins, slowWins, evolvedWins := 0, 0, 0
	var bossTime, evolvedBossTime float64
	for seed := uint64(1); seed <= seeds; seed++ {
		r, res := playBot(seed, 1, 16, 1, false)
		t.Logf("seed=%d phase=%v time=%.1fs kills=%d level=%d minHP=%.0f%% boss=%v phases=%d events=%d enemies=%d embers=%d",
			seed, res.phase, res.time, res.kills, res.level, res.minHealthPct*100, res.bossSpawned, res.bossPhases, res.eventCount, len(res.seenEnemies), r.Payout())
		if res.phase == PhaseVictory {
			wins++
			if res.bossPhases != 2 {
				t.Fatalf("seed %d recorded %d boss phase transitions, want 2 (phases 2 and 3)", seed, res.bossPhases)
			}
			for _, name := range []string{"Ash Mite", "Moth", "Husk", "Thornling", "Wisp", "Shade", "Gloomwarden", "Hollow King"} {
				if !res.seenEnemies[name] {
					t.Fatalf("seed %d never saw %q during the run", seed, name)
				}
			}
		}
		// The same bot after its first win, which unlocks evolution.
		if _, evolved := playBot(seed, 1, 16, 1, true); evolved.phase == PhaseVictory {
			evolvedWins++
			t.Logf("seed=%d evolved %v and felled the Hollow King in %.1fs", seed, evolved.evolved, evolved.bossTime)
			// The fight is compared on the seeds both won, so a hard seed only evolution survives cannot weigh on it.
			if res.phase == PhaseVictory {
				bossTime += res.bossTime
				evolvedBossTime += evolved.bossTime
			}
		}
		// The same bot steering only three times a second stands in for a player who does not dodge well.
		if _, slow := playBot(seed, 1, 16, 20, false); slow.phase == PhaseVictory {
			slowWins++
		}
	}
	if wins < 6 || wins > 8 {
		t.Fatalf("sharp bot won %d/%d seeds, want 6 to 8: good movement should usually win, but not always", wins, seeds)
	}
	if evolvedWins < 6 || evolvedWins > 8 {
		t.Fatalf("sharp bot with evolution won %d/%d seeds, want 6 to 8: evolution should not lift the win rate", evolvedWins, seeds)
	}
	if evolvedBossTime >= bossTime*0.9 {
		t.Fatalf("on the seeds both won, evolved runs fought the Hollow King %.1fs in all and plain runs %.1fs; evolution should clear at least 10%% faster", evolvedBossTime, bossTime)
	}
	if slowWins > 3 {
		t.Fatalf("slow bot won %d/%d seeds, want at most 3: the run should demand movement", slowWins, seeds)
	}

	// Confirm the run is not trivial: standing still with no movement loses well before 12:00.
	afk := NewRun(7, 1)
	for range 12 * 60 * 60 {
		if afk.Snapshot().Phase == PhaseLevelUp {
			afk.Choose(0)
		}
		afk.Step(stepTime, Controls{})
		if afk.Snapshot().Phase == PhaseDefeat {
			break
		}
	}
	if afk.Snapshot().Phase != PhaseDefeat {
		t.Fatalf("standing still survived 12 minutes with %v health; the run is too trivial", afk.Snapshot().Health)
	}

	// Confirm full 12-minute determinism on a fixed seed.
	r1, res1 := playBot(42, 1, 16, 1, false)
	r2, res2 := playBot(42, 1, 16, 1, false)
	if res1.eventCount != res2.eventCount || !reflect.DeepEqual(r1.Snapshot(), r2.Snapshot()) || r1.Payout() != r2.Payout() {
		t.Fatalf("fixed seed 42 did not replay identically: events %d vs %d, embers %d vs %d", res1.eventCount, res2.eventCount, r1.Payout(), r2.Payout())
	}
}

// TestLongNightTiersGetHarder verifies that the sharp bot's win rate falls with every Long Night tier, from about two thirds at tier 1 to a quarter or so at tier 5, as docs/meta-progression.md records, and that a seed, tier and controls replay the same run.
func TestLongNightTiersGetHarder(t *testing.T) {
	const seeds = 100
	tiers := mustGameData().tiers.maxTier()
	if tiers != 5 {
		t.Fatalf("the data defines %d Long Night tiers, want 5", tiers)
	}
	wins := make([]int, tiers+1)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for tier := 1; tier <= tiers; tier++ {
		for seed := uint64(1); seed <= seeds; seed++ {
			wg.Go(func() {
				if _, res := playBot(seed, tier, 16, 1, false); res.phase == PhaseVictory {
					mu.Lock()
					wins[tier]++
					mu.Unlock()
				}
			})
		}
	}
	wg.Wait()
	t.Logf("sharp bot wins by tier over %d seeds: %v", seeds, wins[1:])
	for tier := 2; tier <= tiers; tier++ {
		if wins[tier] >= wins[tier-1] {
			t.Fatalf("tier %d won %d/%d seeds, not fewer than tier %d's %d: every tier should be harder", tier, wins[tier], seeds, tier-1, wins[tier-1])
		}
	}
	if wins[1] < seeds*55/100 || wins[1] > seeds*75/100 {
		t.Fatalf("tier 1 won %d/%d seeds, want about 65%%", wins[1], seeds)
	}
	if wins[tiers] < seeds*15/100 || wins[tiers] > seeds*35/100 {
		t.Fatalf("tier %d won %d/%d seeds, want 20 to 30%%", tiers, wins[tiers], seeds)
	}

	r1, res1 := playBot(42, tiers, 16, 1, false)
	r2, res2 := playBot(42, tiers, 16, 1, false)
	if res1.eventCount != res2.eventCount || !reflect.DeepEqual(r1.Snapshot(), r2.Snapshot()) || r1.Payout() != r2.Payout() {
		t.Fatalf("seed 42 at tier %d did not replay identically: events %d vs %d", tiers, res1.eventCount, res2.eventCount)
	}
}
