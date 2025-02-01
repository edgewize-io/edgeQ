package picker

// NewErrPicker returns a Picker that always returns err on Pick().
func NewErrPicker(err error) Picker {
	return &errPicker{err: err}
}

type errPicker struct {
	err error // Pick() always returns this err.
}

func (p *errPicker) Pick(info PickInfo) (PickResult, error) {
	return PickResult{}, p.err
}
