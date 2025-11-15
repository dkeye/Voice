package core

// Frame is a raw binary payload.
type Frame []byte

// SignalConnection abstracts for a system messaging transport
// Owned by the adapter; the adapter must Close() it.
type SignalConnection interface {
	TrySend(Frame) error
	Close()
}
