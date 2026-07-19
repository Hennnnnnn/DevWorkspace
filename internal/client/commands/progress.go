package commands

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

// spinnerStep adapts actions.StepFunc to the CLI's spinner: each step starts
// a spinner and returns its `done` method as the completion callback.
func spinnerStep(msg string) actions.DoneFunc {
	return startSpinner(msg).done
}

type spinner struct {
	mu     sync.Mutex
	msg    string
	stopCh chan struct{}
}

func startSpinner(msg string) *spinner {
	s := &spinner{msg: msg, stopCh: make(chan struct{})}
	go s.run()
	return s
}

func (s *spinner) run() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-s.stopCh:
			return
		default:
			s.mu.Lock()
			fmt.Fprintf(os.Stderr, "\r%s %s ", frames[i%len(frames)], s.msg)
			s.mu.Unlock()
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func (s *spinner) done(msg string) {
	close(s.stopCh)
	s.mu.Lock()
	fmt.Fprintf(os.Stderr, "\r✓ %s\n", msg)
	s.mu.Unlock()
}

// printStep prints a status line (no spinner). Use for non-blocking info.
func printStep(msg string) {
	fmt.Fprintf(os.Stderr, "  %s\n", msg)
}
