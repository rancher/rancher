package nodedriver

type BoolPointerFlag struct {
	Name   string
	Usage  string
	EnvVar string
}

func (f BoolPointerFlag) String() string {
	return f.Name
}

func (f BoolPointerFlag) Default() interface{} {
	return nil
}
