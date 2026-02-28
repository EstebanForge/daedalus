package providers

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestACPEventMappingGoldenSessionUpdate(t *testing.T) {
	t.Parallel()

	provider := acpProvider{providerKey: "test"}
	cases := []struct {
		name        string
		update      acpSessionUpdate
		wantEvents  []Event
		wantSummary string
	}{
		{
			name:        "assistant content",
			update:      acpSessionUpdate{Content: "hello world"},
			wantEvents:  []Event{{Type: EventAssistantText, Message: "hello world"}},
			wantSummary: "hello world",
		},
		{
			name:       "tool started",
			update:     acpSessionUpdate{ToolName: "shell", Event: "start"},
			wantEvents: []Event{{Type: EventToolStarted, Message: "shell"}},
		},
		{
			name:       "tool finished",
			update:     acpSessionUpdate{Tool: "shell", Status: "completed"},
			wantEvents: []Event{{Type: EventToolFinished, Message: "shell"}},
		},
		{
			name:        "assistant and tool update",
			update:      acpSessionUpdate{Text: "running", ToolName: "shell", Event: "running"},
			wantEvents:  []Event{{Type: EventAssistantText, Message: "running"}, {Type: EventToolStarted, Message: "shell"}},
			wantSummary: "running",
		},
		{
			name:       "unknown update shape",
			update:     acpSessionUpdate{Type: "unknown"},
			wantEvents: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := mustMarshalJSON(tc.update)
			events := make(chan Event, 8)
			var summary strings.Builder

			provider.handleSessionUpdate(raw, events, &summary)
			close(events)

			gotEvents := collectEvents(events)
			if len(tc.wantEvents) == 0 && len(gotEvents) == 0 {
				// Treat nil and empty slices as equivalent for golden assertions.
			} else if !reflect.DeepEqual(tc.wantEvents, gotEvents) {
				t.Fatalf("events mismatch\ngot:  %+v\nwant: %+v", gotEvents, tc.wantEvents)
			}
			if got := summary.String(); got != tc.wantSummary {
				t.Fatalf("summary mismatch: got %q want %q", got, tc.wantSummary)
			}
		})
	}
}

func TestACPEventMappingGoldenPromptResult(t *testing.T) {
	t.Parallel()

	provider := acpProvider{providerKey: "test"}
	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{
			name: "output blocks",
			raw:  mustMarshalJSON(acpPromptResult{Output: []acpContentBlock{{Type: "text", Text: "hello"}}}),
			want: "hello",
		},
		{
			name: "content blocks",
			raw:  mustMarshalJSON(acpPromptResult{Content: []acpContentBlock{{Type: "text", Text: "world"}}}),
			want: "world",
		},
		{
			name: "message fallback",
			raw:  mustMarshalJSON(acpPromptResult{Message: "done"}),
			want: "done",
		},
		{
			name: "invalid payload",
			raw:  json.RawMessage(`{"output":1}`),
			want: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := provider.extractContentFromPromptResult(tc.raw)
			if got != tc.want {
				t.Fatalf("extractContentFromPromptResult: got %q want %q", got, tc.want)
			}
		})
	}
}
