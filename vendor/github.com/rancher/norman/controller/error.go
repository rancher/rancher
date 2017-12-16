package controller

type ForgetError struct {
	Err error
}

func (f *ForgetError) Error() string {
	return f.Err.Error()
}
