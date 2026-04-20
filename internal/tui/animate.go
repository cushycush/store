package tui

import (
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// AnimationFPS is the tick rate while any animation is in flight.
// 40Hz is enough for perceptible motion without straining slow terminals.
const AnimationFPS = 40

// tickInterval is derived from AnimationFPS.
var tickInterval = time.Second / AnimationFPS

// FrameMsg fires on each animation tick.
type FrameMsg time.Time

// Tick returns a tea.Cmd that fires the next FrameMsg.
func Tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return FrameMsg(t)
	})
}

// Easings. We use quartic ease-out for reveals and quintic ease-out for
// secondary reveals. No bounce, no elastic — decelerations that match
// how physical objects come to rest.

func easeOutQuart(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	x := 1 - t
	return 1 - x*x*x*x
}

func easeOutQuint(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	x := 1 - t
	return 1 - x*x*x*x*x
}

// reveal0to1 returns the reveal opacity for an item at index i given total
// item count n, how long since the reveal started, and the stagger between
// consecutive items.
func revealAt(i, n int, elapsed, stagger, dur time.Duration) float64 {
	if n <= 0 || i >= n {
		return 1
	}
	start := time.Duration(i) * stagger
	if elapsed < start {
		return 0
	}
	t := float64(elapsed-start) / float64(dur)
	return easeOutQuart(t)
}

// decay returns 1 at t=0 and 0 at t >= dur, easing out.
func decay(age, dur time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	if age >= dur {
		return 0
	}
	return 1 - easeOutQuint(float64(age)/float64(dur))
}

// pulse returns a 0..1 sine-wave pulse with the given period, phased so
// that pulse(now=0) = 0.
func pulse(at time.Time, period time.Duration) float64 {
	phase := float64(at.UnixNano()%int64(period)) / float64(period)
	return 0.5 - 0.5*math.Cos(phase*2*math.Pi)
}
