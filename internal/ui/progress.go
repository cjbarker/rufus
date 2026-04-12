package ui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const progressBarWidth = 30

// Progress displays an animated progress bar with stats.
type Progress struct {
	total     int64
	current   atomic.Int64
	label     string
	startTime time.Time
	mu        sync.Mutex
	done      chan struct{}
	stopped   bool
}

// NewProgress creates a new progress bar.
func NewProgress(label string, total int64) *Progress {
	return &Progress{
		total:     total,
		label:     label,
		startTime: time.Now(),
		done:      make(chan struct{}),
	}
}

// Start begins the progress bar animation.
func (p *Progress) Start() {
	if quietMode {
		return
	}
	if !IsTTY() {
		fmt.Printf("%s (0/%d)\n", p.label, p.total)
		return
	}

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				p.render()
			}
		}
	}()
}

// Increment advances the progress bar by 1.
func (p *Progress) Increment() {
	p.current.Add(1)
}

// SetCurrent sets the current progress value.
func (p *Progress) SetCurrent(n int64) {
	p.current.Store(n)
}

func (p *Progress) render() {
	current := p.current.Load()
	total := p.total
	if total == 0 {
		total = 1
	}

	pct := float64(current) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}

	filled := int(pct * float64(progressBarWidth))
	empty := progressBarWidth - filled

	bar := SuccessStyle.Render(strings.Repeat("█", filled)) +
		Dim.Render(strings.Repeat("░", empty))

	elapsed := time.Since(p.startTime)
	rate := float64(0)
	if elapsed.Seconds() > 0 {
		rate = float64(current) / elapsed.Seconds()
	}

	eta := ""
	if rate > 0 && current < total {
		remaining := float64(total-current) / rate
		eta = fmt.Sprintf(" ETA %s", (time.Duration(remaining) * time.Second).Round(time.Second))
	}

	pctStr := fmt.Sprintf("%3.0f%%", pct*100)
	stats := Dim.Render(fmt.Sprintf(" %d/%d  %.1f/s%s", current, total, rate, eta))

	fmt.Printf("\r\033[K  %s %s %s%s", p.label, bar, Highlight.Render(pctStr), stats)
}

// Stop ends the progress bar animation.
func (p *Progress) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true
	close(p.done)

	if IsTTY() {
		p.render()
		fmt.Println()
	}
}
