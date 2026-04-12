package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner frames for smooth animation.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with a message on the terminal.
type Spinner struct {
	message string
	mu      sync.Mutex
	done    chan struct{}
	stopped bool
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation. Call Stop() to end it.
func (s *Spinner) Start() {
	if quietMode {
		return
	}
	if !IsTTY() {
		fmt.Printf("%s...\n", s.message)
		return
	}

	go func() {
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()

				frame := InfoStyle.Render(spinnerFrames[i%len(spinnerFrames)])
				fmt.Printf("\r\033[K  %s %s", frame, msg)
				i++
			}
		}
	}()
}

// UpdateMessage changes the spinner message while it's running.
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// Stop ends the spinner animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.done)

	if IsTTY() {
		fmt.Print("\r\033[K")
	}
}

// StopWithMessage ends the spinner and prints a final message.
func (s *Spinner) StopWithMessage(msg string) {
	s.Stop()
	fmt.Println(msg)
}

// StopWithSuccess ends the spinner and prints a success message with a checkmark.
func (s *Spinner) StopWithSuccess(msg string) {
	s.StopWithMessage(fmt.Sprintf("  %s %s", SuccessStyle.Render("✔"), msg))
}

// StopWithError ends the spinner and prints an error message with an X.
func (s *Spinner) StopWithError(msg string) {
	s.StopWithMessage(fmt.Sprintf("  %s %s", ErrorStyle.Render("✖"), msg))
}
