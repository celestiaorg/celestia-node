package utils

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SetStatusAndEnd sets the status of the span depending on the contents of the passed error and
// ends it.
func SetStatusAndEnd(span trace.Span, err error) {
	defer span.End()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return
	}
	span.SetStatus(codes.Ok, "")
}
