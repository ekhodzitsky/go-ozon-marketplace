package errors

import "go.uber.org/zap"

// Fields extracts structured zap fields from an error. Adapters can attach
// these fields to a logger near the call site without knowing the transport.
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
