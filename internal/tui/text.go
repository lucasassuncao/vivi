package tui

import "time"

// Timers the parent owns, because what they drive is the parent's: the banner
// and the header's countdown. Each is a variable so tests can shrink it: a timer
// does not answer inside the harness's wait, which used to give up silently
// 1102 times a run. The caret's own timer is ui.BlinkSpeed, beside the input.

var (
	// bannerLife is how long a transient footer message stays up.
	bannerLife = 6 * time.Second

	// clockInterval brings the render around so the header's token countdown
	// moves on its own. Well inside the countdown's own resolution, which is
	// minutes, at a cost of two wake-ups a minute.
	clockInterval = 30 * time.Second
)
