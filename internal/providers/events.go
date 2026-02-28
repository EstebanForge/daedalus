package providers

import "strings"

func pushProviderEvent(events chan Event, eventType EventType, message string) {
	if events == nil {
		return
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	events <- Event{Type: eventType, Message: trimmed}
}
