package errors

// GraphQLError — транспортно-нейтральное представление GraphQL-ошибки.
// Не зависит от конкретного фреймворка: api-gateway адаптирует её на стыке.
type GraphQLError struct {
	Message    string
	Extensions map[string]any
}

// ToGraphQLError превращает доменную ошибку в GraphQL-объект. В extensions кладёт
// code, publicKey, retryable и detail, чтобы клиенты не парсили человеческий текст.
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
