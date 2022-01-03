// Package pretty formats types nicely
package pretty

import (
	"fmt"
	"time"
)

// Duration formats the duration as HH:MM:SS, MM:SS
func Duration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s :=  d / time.Second

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	} else {
		return fmt.Sprintf("%d:%02d", m, s)
	}
}