package providers

func collectEvents(events <-chan Event) []Event {
	if events == nil {
		return nil
	}

	collected := make([]Event, 0)
	for event := range events {
		collected = append(collected, event)
	}
	return collected
}

func eventTypes(events []Event) []EventType {
	types := make([]EventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
