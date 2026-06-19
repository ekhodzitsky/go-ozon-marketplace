package errors

// Kind is the canonical transport-neutral classification for domain errors.
// It sits at the seam between the domain vocabulary and transport adapters,
// allowing gRPC, HTTP, GraphQL, and logging to leverage a single source of truth.
type Kind string

const (
	KindNotFound           Kind = "not_found"
	KindAlreadyExists      Kind = "already_exists"
	KindInvalidArgument    Kind = "invalid_argument"
	KindInvalidCredentials Kind = "invalid_credentials"
	KindPermissionDenied   Kind = "permission_denied"
	KindConflict           Kind = "conflict"
	KindInsufficientStock  Kind = "insufficient_stock"
	KindFailedPrecondition Kind = "failed_precondition"
	KindUnauthenticated    Kind = "unauthenticated"
	KindUnavailable        Kind = "unavailable"
	KindDeadlineExceeded   Kind = "deadline_exceeded"
	KindInternal           Kind = "internal"
)
