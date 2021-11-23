package ms

// =============================================================================
// Message Queue

type MQueue struct {
	name       string
	messages   []*Message
	lastReaded int
}
