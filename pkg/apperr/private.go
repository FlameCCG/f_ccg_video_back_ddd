package apperr

// PrivateError keeps a sensitive source error inspectable without printing its untrusted response body.
type PrivateError struct {
	Operation string
	Cause     error
}

func (e *PrivateError) Error() string {
	if e.Cause == nil {
		return e.Operation
	}

	return e.Cause.Error()
}
func (e *PrivateError) Unwrap() error { return e.Cause }

// LogMessage excludes untrusted source response text from logging.
func (e *PrivateError) LogMessage() string {
	return e.Operation + ": sensitive upstream response omitted"
}

// Private retains a sensitive source cause while providing a safe output message.
func Private(operation string, cause error) error {
	if cause == nil {
		return nil
	}

	return &PrivateError{Operation: operation, Cause: cause}
}
