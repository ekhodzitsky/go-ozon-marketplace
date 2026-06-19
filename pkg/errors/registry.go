package errors

import (
	"net/http"

	"google.golang.org/grpc/codes"
)

// Entry is the adapter-neutral mapping for a Kind. Adapters leverage this
// table instead of encoding transport-specific rules inside each error path.
type Entry struct {
	GRPCCode   codes.Code
	HTTPStatus int
	PublicKey  string
	Retryable  bool
}

// registry is the single source of truth for Kind metadata. Keeping it in one
// place maximizes locality: a change to classification semantics happens here
// and propagates to gRPC, HTTP, GraphQL, and logs automatically.
var registry = map[Kind]Entry{
	KindNotFound:           {GRPCCode: codes.NotFound, HTTPStatus: http.StatusNotFound, PublicKey: "NOT_FOUND", Retryable: false},
	KindAlreadyExists:      {GRPCCode: codes.AlreadyExists, HTTPStatus: http.StatusConflict, PublicKey: "ALREADY_EXISTS", Retryable: false},
	KindInvalidArgument:    {GRPCCode: codes.InvalidArgument, HTTPStatus: http.StatusBadRequest, PublicKey: "INVALID_ARGUMENT", Retryable: false},
	KindInvalidCredentials: {GRPCCode: codes.Unauthenticated, HTTPStatus: http.StatusUnauthorized, PublicKey: "INVALID_CREDENTIALS", Retryable: false},
	KindPermissionDenied:   {GRPCCode: codes.PermissionDenied, HTTPStatus: http.StatusForbidden, PublicKey: "PERMISSION_DENIED", Retryable: false},
	KindConflict:           {GRPCCode: codes.FailedPrecondition, HTTPStatus: http.StatusConflict, PublicKey: "CONFLICT", Retryable: false},
	KindInsufficientStock:  {GRPCCode: codes.FailedPrecondition, HTTPStatus: http.StatusConflict, PublicKey: "INSUFFICIENT_STOCK", Retryable: false},
	KindFailedPrecondition: {GRPCCode: codes.FailedPrecondition, HTTPStatus: http.StatusConflict, PublicKey: "FAILED_PRECONDITION", Retryable: false},
	KindUnauthenticated:    {GRPCCode: codes.Unauthenticated, HTTPStatus: http.StatusUnauthorized, PublicKey: "UNAUTHENTICATED", Retryable: false},
	KindUnavailable:        {GRPCCode: codes.Unavailable, HTTPStatus: http.StatusServiceUnavailable, PublicKey: "UNAVAILABLE", Retryable: true},
	KindDeadlineExceeded:   {GRPCCode: codes.DeadlineExceeded, HTTPStatus: http.StatusGatewayTimeout, PublicKey: "DEADLINE_EXCEEDED", Retryable: true},
	KindInternal:           {GRPCCode: codes.Internal, HTTPStatus: http.StatusInternalServerError, PublicKey: "INTERNAL", Retryable: false},
}

// EntryFor returns the adapter metadata for a Kind.
func EntryFor(kind Kind) (Entry, bool) {
	entry, ok := registry[kind]
	return entry, ok
}

// Kinds returns every registered Kind. Useful for completeness checks.
func Kinds() []Kind {
	kinds := make([]Kind, 0, len(registry))
	for kind := range registry {
		kinds = append(kinds, kind)
	}
	return kinds
}

// ParseKind converts a string into a Kind when it is registered; otherwise it
// falls back to KindInternal so unknown classifications never leak as empty.
func ParseKind(s string) Kind {
	if _, ok := registry[Kind(s)]; ok {
		return Kind(s)
	}
	return KindInternal
}
