package build

import (
	"fmt"
	"time"
)

// FormatDuration returns a human-readable duration string
// - "450ms" for durations < 1 second
// - "2.3s" for durations 1-60 seconds
// - "1m 12s" for durations >= 60 seconds
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	if d < 60*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) - minutes*60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
