package dev

import (
	"fmt"
	"net/http"
	"sync"
)

// EventPath is where the reload client connects. It is namespaced under `_gsx`
// so it cannot collide with an application route.
const EventPath = "/_gsx/live"

// Event is what the browser is told to do.
type Event struct {
	// Kind is "reload" on a successful rebuild, or "error" when the build
	// failed — in which case Message holds the compiler output to display.
	Kind    string
	Message string
}

// Broker fans one build outcome out to every connected browser tab.
type Broker struct {
	mu      sync.Mutex
	clients map[chan Event]struct{}
	// last is replayed to a tab that connects after a failed build, so a
	// reconnect during a broken build still shows the error rather than a
	// blank overlay.
	last *Event
}

func NewBroker() *Broker {
	return &Broker{clients: map[chan Event]struct{}{}}
}

func (b *Broker) Publish(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ev.Kind == "error" {
		b.last = &ev
	} else {
		b.last = nil
	}

	for ch := range b.clients {
		// A tab that is not keeping up is mid-reload and about to reconnect,
		// so dropping its event is harmless and must not block the build loop.
		select {
		case ch <- ev:
		default:
		}
	}
}

func (b *Broker) subscribe() (chan Event, *Event) {
	ch := make(chan Event, 4)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[ch] = struct{}{}
	return ch, b.last
}

func (b *Broker) unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

// ServeHTTP streams events to one browser tab over SSE.
//
// SSE rather than a WebSocket: it is one-directional, which is all a reload
// signal needs, and the browser reconnects on its own — so a tab left open
// across a server restart recovers without the user touching it.
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch, replay := b.subscribe()
	defer b.unsubscribe(ch)

	write := func(ev Event) bool {
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, jsonString(ev.Message)); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Tell the client it is connected before anything else, so the overlay can
	// distinguish "server up" from "server still starting".
	if !write(Event{Kind: "connected"}) {
		return
	}
	if replay != nil && !write(*replay) {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok || !write(ev) {
				return
			}
		}
	}
}

// jsonString quotes a string for an SSE data field. Newlines would otherwise
// terminate the field, so they are escaped along with the usual JSON set.
func jsonString(s string) string {
	var out []byte
	out = append(out, '"')
	for _, r := range s {
		switch r {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if r < 0x20 {
				out = append(out, []byte(fmt.Sprintf("\\u%04x", r))...)
				continue
			}
			out = append(out, []byte(string(r))...)
		}
	}
	return string(append(out, '"'))
}
