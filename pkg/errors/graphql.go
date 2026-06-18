package errors

// GraphQLError is a transport-agnostic representation of a GraphQL error. It
// deliberately avoids a dependency on any GraphQL framework; the api-gateway
// can adapt it to its chosen library at the seam.
type GraphQLError struct {
	Message    string
	Extensions map[string]any
}

// ToGraphQLError maps a domain error to a GraphQL-shaped value. It produces
// extensions with code, publicKey, retryable, and detail so clients can react
// without parsing human-readable messages.
func ToGraphQLError(err error) *GraphQLError {
	if err == nil {
		return nil
	}

	ae, ok := err.(*AppError)
	if !ok {
		ae = FromStatus(ToStatus(err))
	}

	entry, ok := EntryFor(ae.Kind)
	if !ok {
		entry, _ = EntryFor(KindInternal)
	}

	return &GraphQLError{
		Message: ae.Message,
		Extensions: map[string]any{
			"code":      entry.GRPCCode.String(),
			"publicKey": entry.PublicKey,
			"retryable": entry.Retryable,
			"detail":    ae.Detail,
		},
	}
}
