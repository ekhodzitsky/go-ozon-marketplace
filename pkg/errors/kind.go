package errors

// Kind — каноническая транспортно-нейтральная классификация доменных ошибок.
// Служит стыком между доменным словарём и транспортными адаптерами:
// gRPC, HTTP, GraphQL и логи используют одну и ту же таблицу.
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
