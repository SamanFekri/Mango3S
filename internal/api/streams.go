package api

import (
	"mango3s/internal/streams"
)

// Change stream viewer (Explorer › Watch changes).

// StreamEvents is pushed as "stream:events" while a viewer is running.
type StreamEvents struct {
	ID          string          `json:"id"`
	Events      []streams.Event `json:"events"`
	ResumeToken string          `json:"resumeToken"`
}

// StartChangeStream starts (or, with a resume token, resumes) the stream of
// viewer id. Events arrive as "stream:events", state changes as
// "stream:state".
func (a *App) StartChangeStream(id string, r streams.Request) error {
	return a.encode("StartChangeStream", a.streams.Start(id, r))
}

// StopChangeStream pauses viewer id; its last resume token was reported.
func (a *App) StopChangeStream(id string) { a.streams.Stop(id) }
