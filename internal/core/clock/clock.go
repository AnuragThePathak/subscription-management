package clock

import "time"

// NowFn returns the current time. In production, pass time.Now.
// In tests, pass a function returning a fixed time for deterministic assertions.
type NowFn func() time.Time
