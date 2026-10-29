package identity

type ValidationError struct {
	Message string
}

func ErrValidation(msg string) error {
	return ValidationError{Message: msg}
}

func (e ValidationError) Error() string {
	return e.Message
}
