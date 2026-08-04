// Package bus is an in-process topic fan-out feeding the SSE endpoints.
package bus

import "sync"

type Message struct {
	Topic string `json:"topic"`
	Kind  string `json:"kind"`
	Data  any    `json:"data"`
}

type Bus struct {
	mu   sync.RWMutex
	subs map[string]map[chan Message]struct{}
}

func New() *Bus { return &Bus{subs: map[string]map[chan Message]struct{}{}} }

// Subscribe returns a buffered channel and the func that closes it.
func (b *Bus) Subscribe(topic string) (<-chan Message, func()) {
	ch := make(chan Message, 64)
	b.mu.Lock()
	if b.subs[topic] == nil {
		b.subs[topic] = map[chan Message]struct{}{}
	}
	b.subs[topic][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if set, ok := b.subs[topic]; ok {
			if _, live := set[ch]; live {
				delete(set, ch)
				close(ch)
			}
			if len(set) == 0 {
				delete(b.subs, topic)
			}
		}
	}
}

// Publish never blocks: a subscriber that cannot keep up drops messages rather
// than stalling an agent run.
func (b *Bus) Publish(topic, kind string, data any) {
	msg := Message{Topic: topic, Kind: kind, Data: data}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs[topic] {
		select {
		case ch <- msg:
		default:
		}
	}
	for ch := range b.subs["*"] {
		select {
		case ch <- msg:
		default:
		}
	}
}
