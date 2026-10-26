package baseerror

func Glean(fns ...func() error) error {
	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func NewDefaultError(desc string) *CodeError {
	return NewCodeError(10001, desc)
}
