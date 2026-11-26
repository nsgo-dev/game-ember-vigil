package main

import (
	"math"
	"testing"
)

// TestSpawnPosIsInsideTheArenaAndOffscreen checks the ring from the arena's corners and its middle, where clamping has the least room.
func TestSpawnPosIsInsideTheArenaAndOffscreen(t *testing.T) {
	r := quietRun(t)
	arena := Vec2{r.data.run.Arena.Width, r.data.run.Arena.Height}
	radius := r.maxRadius
	for _, keeperPos := range []Vec2{
		{arena.X / 2, arena.Y / 2},
		{radius, radius},
		{arena.X - radius, radius},
		{radius, arena.Y - radius},
		{arena.X - radius, arena.Y - radius},
	} {
		r.keeper.pos = keeperPos
		spawned := 0
		for range 200 {
			p, ok := r.spawnPos(radius)
			if !ok {
				continue
			}
			spawned++
			if p.X < radius || p.Y < radius || p.X > arena.X-radius || p.Y > arena.Y-radius {
				t.Fatalf("a spawn at %+v from keeper %+v lies outside the arena", p, keeperPos)
			}
			if !offscreen(keeperPos, p) {
				t.Fatalf("a spawn at %+v from keeper %+v lies in the view", p, keeperPos)
			}
		}
		if spawned == 0 {
			t.Fatalf("the ring found no spawn point with the keeper at %+v", keeperPos)
		}
	}
}

// TestDirectorFollowsTheTimeline checks that the clock selects the span whose From it has passed.
func TestDirectorFollowsTheTimeline(t *testing.T) {
	r := quietRun(t)
	for i := range r.data.waves.Spans {
		span := &r.data.waves.Spans[i]
		r.director.nextSpawn = math.Inf(1)
		if got := r.director.spanAt(span.From); got != span {
			t.Fatalf("at %v seconds the director is in %+v, want the span starting then", span.From, got)
		}
		if span.From > 0 {
			if got := r.director.spanAt(span.From - 0.001); got != &r.data.waves.Spans[i-1] {
				t.Fatalf("just before %v seconds the director is in %+v, want the span before", span.From, got)
			}
		}
	}
}

// TestDirectorAppliesTheSpanItSpawnsFrom checks that a spawn takes its health and damage from the span's multipliers.
func TestDirectorAppliesTheSpanItSpawnsFrom(t *testing.T) {
	r := quietRun(t)
	span := &r.data.waves.Spans[2]
	r.time = span.From
	r.director.bossSpawned = false
	r.director.nextSpawn = span.From
	r.director.step(r)
	if len(r.enemies) != 1 {
		t.Fatalf("the director spawned %d enemies, want 1", len(r.enemies))
	}
	e := &r.enemies[0]
	def := &r.data.enemies.Enemies[e.kind]
	if want := def.Health * span.Health; math.Abs(e.maxHP-want) > 1e-9 {
		t.Fatalf("the spawn has %v health, want %v", e.maxHP, want)
	}
	if want := def.Damage * span.Damage; math.Abs(e.damage-want) > 1e-9 {
		t.Fatalf("the spawn deals %v damage, want %v", e.damage, want)
	}
	if want := span.Every; math.Abs(r.director.nextSpawn-(span.From+want)) > 1e-9 {
		t.Fatalf("the next spawn is due at %v, want %v", r.director.nextSpawn, span.From+want)
	}
}

// TestDirectorWaitsAtTheAliveCap checks that a full span spawns nothing and keeps the due spawn waiting.
func TestDirectorWaitsAtTheAliveCap(t *testing.T) {
	r := quietRun(t)
	span := &r.data.waves.Spans[0]
	r.time = 10
	r.director.bossSpawned = false
	r.director.nextSpawn = 0
	for len(r.enemies) < span.MaxAlive {
		r.addEnemy(0, r.keeper.pos.Add(Vec2{X: 100, Y: 100}), 1, 1)
	}
	r.director.step(r)
	if len(r.enemies) != span.MaxAlive {
		t.Fatalf("the director spawned past the cap of %d to %d", span.MaxAlive, len(r.enemies))
	}
	if r.director.nextSpawn != 0 {
		t.Fatalf("the due spawn moved to %v while the cap was full, want it waiting at 0", r.director.nextSpawn)
	}

	r.enemies = r.enemies[:span.MaxAlive-2]
	r.director.step(r)
	if len(r.enemies) <= span.MaxAlive-2 {
		t.Fatal("the director did not catch up once the cap had room")
	}
	if r.director.nextSpawn <= 0 {
		t.Fatalf("the caught-up spawn did not move the next one past %v", r.director.nextSpawn)
	}
}

// TestDirectorTriggersSurgesElitesAndPausesForBoss checks scheduled surges, Gloomwarden elites, and that normal spawns pause once the Hollow King arrives at 12:00.
func TestDirectorTriggersSurgesElitesAndPausesForBoss(t *testing.T) {
	r := NewRun(19)
	r.director.nextSpawn = math.Inf(1)

	// Trigger the first surge.
	firstSurge := r.data.waves.Surges[0]
	r.time = firstSurge.At
	r.director.step(r)
	if len(r.enemies) == 0 {
		t.Fatalf("surge at %vs spawned no enemies", firstSurge.At)
	}

	// Trigger the first elite (Gloomwarden).
	firstElite := r.data.waves.Elites[0]
	r.time = firstElite.At
	r.director.step(r)
	foundElite := false
	for _, e := range r.enemies {
		if e.elite && r.data.enemies.Enemies[e.kind].Name == firstElite.Enemy {
			foundElite = true
			break
		}
	}
	if !foundElite {
		t.Fatalf("elite %q did not spawn at %vs", firstElite.Enemy, firstElite.At)
	}

	// Reach 12:00: Hollow King spawns, clears wave enemies, and pauses normal spawns.
	r.director.nextSpawn = r.data.waves.Boss.At
	r.time = r.data.waves.Boss.At
	r.director.step(r)
	if !r.boss.active {
		t.Fatal("Hollow King did not spawn at 12:00")
	}
	if len(r.enemies) != 1 || !r.enemies[0].isBoss {
		t.Fatalf("expected only the Hollow King on spawn, got %d enemies", len(r.enemies))
	}
	if !hasEvent(r.Events(), EventBossSpawn) {
		t.Fatal("boss arrival emitted no EventBossSpawn")
	}

	// Further director steps during the boss fight must not spawn regular wave enemies.
	r.time += 10
	r.director.step(r)
	if len(r.enemies) != 1 {
		t.Fatalf("normal spawns did not pause during boss fight: %d enemies alive", len(r.enemies))
	}
}

// TestDirectorSpawnsSurroundWaveAroundKeeper checks that a surround surge places enemies across all four quadrants around the keeper.
func TestDirectorSpawnsSurroundWaveAroundKeeper(t *testing.T) {
	r := NewRun(23)
	r.director.nextSpawn = math.Inf(1)
	r.director.nextElite = len(r.data.waves.Elites)

	var surround *surgeDef
	for i := range r.data.waves.Surges {
		if r.data.waves.Surges[i].Surround {
			surround = &r.data.waves.Surges[i]
			r.director.nextSurge = i
			break
		}
	}
	if surround == nil {
		t.Fatal("data/waves.yaml defines no surround surge")
	}
	r.time = surround.At
	r.director.step(r)
	if len(r.enemies) != surround.Count {
		t.Fatalf("surround surge spawned %d enemies, want %d", len(r.enemies), surround.Count)
	}
	var q [4]bool
	for _, e := range r.enemies {
		d := e.pos.Sub(r.keeper.pos)
		switch {
		case d.X >= 0 && d.Y >= 0:
			q[0] = true
		case d.X < 0 && d.Y >= 0:
			q[1] = true
		case d.X < 0 && d.Y < 0:
			q[2] = true
		default:
			q[3] = true
		}
	}
	for i, seen := range q {
		if !seen {
			t.Fatalf("surround surge left quadrant %d empty around keeper", i)
		}
	}
}
