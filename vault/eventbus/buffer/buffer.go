package buffer

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/eventlogger"
)

var _ eventlogger.Node = (*replayBuffer)(nil)

type replayBuffer struct {
	mtx      sync.RWMutex
	capacity int   // Maximum number of events to store.
	length   int   // Current number of events stored.
	head     *node // Most recent event added.
	tail     *node // Start point for replays.
}

type ReplayBuffer interface {
	Replay(ctx context.Context, nodes []eventlogger.Node) error
}

type node struct {
	next  *node
	event *eventlogger.Event
}

func NewReplayBuffer(capacity int) *replayBuffer {
	head := &node{}
	return &replayBuffer{
		capacity: capacity,
		head:     head,
		tail:     &node{next: head},
	}
}

func (b *replayBuffer) Process(_ context.Context, event *eventlogger.Event) (*eventlogger.Event, error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	nextHead := &node{}
	b.head.event = event
	b.head.next = nextHead
	b.head = nextHead
	b.length++

	if b.length > b.capacity {
		// Some in-flight replays may still hold references to the old tail, but
		// it will get garbage collected once they're finished.
		b.tail = b.tail.next
		b.length--
	}

	return event, nil
}

func (node *replayBuffer) Reopen() error {
	return nil
}

func (node *replayBuffer) Type() eventlogger.NodeType {
	return eventlogger.NodeTypeSink
}

func (b *replayBuffer) Replay(ctx context.Context, nodes []eventlogger.Node) error {
	b.mtx.RLock()
	current := b.tail.next
	count := b.length
	if current.event == nil {
		return nil
	}
	b.mtx.RUnlock()

	for i := 0; i < count && current != nil; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		currentEvent := current.event
		if currentEvent == nil {
			current = current.next
			continue
		}
		var err error
		for _, node := range nodes {
			currentEvent, err = node.Process(ctx, currentEvent)
			if err != nil {
				return fmt.Errorf("error while replaying events: %w", err)
			}
			// If the returned event is nil, it got filtered out, so we're finished processing.
			if currentEvent == nil {
				break
			}
		}

		current = current.next
	}

	return nil
}
