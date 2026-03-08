package stream

// Event is a single item emitted by a streaming harness.
type Event struct {
	Content string
	Done    bool
	Err     error
}
