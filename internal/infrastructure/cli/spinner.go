package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Spinner displays an animated spinner during long operations
type Spinner struct {
	frames   []string
	interval time.Duration
	writer   io.Writer
	stopChan chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// NewSpinner creates a new spinner
func NewSpinner(w io.Writer) *Spinner {
	return &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		writer:   w,
		stopChan: make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		idx := 0
		for {
			select {
			case <-s.stopChan:
				// Clear the spinner line
				fmt.Fprintf(s.writer, "\r\033[K")
				return
			default:
				fmt.Fprintf(s.writer, "\r%s ", s.frames[idx%len(s.frames)])
				idx++
				time.Sleep(s.interval)
			}
		}
	}()
}

// Stop stops the spinner animation
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
}
