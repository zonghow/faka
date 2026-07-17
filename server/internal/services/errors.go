package services

type ServiceError struct {
	Message string
}

func (e *ServiceError) Error() string { return e.Message }

func Err(msg string) error { return &ServiceError{Message: msg} }

type FileClaimConflict struct{}

func (FileClaimConflict) Error() string { return "file claim conflict" }
