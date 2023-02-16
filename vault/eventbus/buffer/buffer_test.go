package buffer

import (
	"context"
	"testing"

	"github.com/hashicorp/eventlogger"
)

func TestReplayBuffer(t *testing.T) {
	a := &eventlogger.Event{Type: "a"}
	b := &eventlogger.Event{Type: "b"}
	c := &eventlogger.Event{Type: "c"}
	d := &eventlogger.Event{Type: "d"}
	e := &eventlogger.Event{Type: "e"}
	f := &eventlogger.Event{Type: "f"}
	for name, tc := range map[string]struct {
		eventsIn, eventsExpected []*eventlogger.Event
	}{
		"zero":            {[]*eventlogger.Event{}, []*eventlogger.Event{}},
		"one":             {[]*eventlogger.Event{a}, []*eventlogger.Event{a}},
		"capacity":        {[]*eventlogger.Event{a, b, c}, []*eventlogger.Event{a, b, c}},
		"over capacity":   {[]*eventlogger.Event{a, b, c, d}, []*eventlogger.Event{b, c, d}},
		"over capacity 2": {[]*eventlogger.Event{a, b, c, d, e, f}, []*eventlogger.Event{d, e, f}},
	} {
		t.Run(name, func(t *testing.T) {
			b := NewReplayBuffer(3)
			ctx := context.Background()
			for _, event := range tc.eventsIn {
				_, _ = b.Process(ctx, event)
			}

			var actual []eventlogger.Event
			nodes := []eventlogger.Node{&eventlogger.Filter{Predicate: func(e *eventlogger.Event) (bool, error) {
				actual = append(actual, *e)
				return true, nil
			}}}

			b.Replay(ctx, nodes)

			if len(tc.eventsExpected) != len(actual) {
				t.Fatalf("Expected %d events, got %d", len(tc.eventsExpected), len(actual))
			}

			for i := 0; i < len(tc.eventsExpected); i++ {
				if tc.eventsExpected[i].Type != actual[i].Type {
					t.Fatalf("Expected index %d to be %v, got %v", i, tc.eventsExpected[i], actual[i])
				}
			}
		})
	}
}
