package main

import "time"

// run is one run of the game. The shell steps it one fixed step at a time and draws it; it knows nothing about views, input devices, sound, storage or the wall clock.
type run struct {
	// elapsed is how long the run has lasted, which stops with the run when a screen pauses it.
	elapsed time.Duration
}

// newRun starts a run at 00:00.
func newRun() *run { return &run{} }

// step moves the run on by one fixed step.
func (r *run) step(dt time.Duration) { r.elapsed += dt }
