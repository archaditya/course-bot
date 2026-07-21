package provider

// IDGenerator generates unique identifiers. The interface exists so the
// application layer never imports crypto/rand directly — it depends only on
// domain interfaces, not infrastructure. This keeps use cases testable with
// deterministic IDs in unit tests.
//
// The single production implementation is infrastructure/id.UUIDGenerator.
type IDGenerator interface {
	New() string
}
