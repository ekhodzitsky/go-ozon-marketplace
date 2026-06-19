package errors

import "go.uber.org/zap"

// Fields вытаскивает из ошибки структурированные поля для zap.
// Адаптеры могут прикреплять их к логгеру рядом с местом вызова, не зная про транспорт.
func Fields(err error) []zap.Field {
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

	return []zap.Field{
		zap.String("kind", string(ae.Kind)),
		zap.String("code", ae.Code),
		zap.String("message", ae.Message),
		zap.String("detail", ae.Detail),
		zap.Bool("retryable", entry.Retryable),
		zap.Error(err),
	}
}
