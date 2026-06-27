package observability

import "context"

const RequestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value := ctx.Value(requestIDContextKey{})
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}
