package nodedriver

type BoolPointerFlag struct {
	Name     string
	Usage    string
	EnvVar   string
	Optional bool
}

func (f BoolPointerFlag) String() string {
	return f.Name
}

func (f BoolPointerFlag) Default() interface{} {
	return nil
}

func (f BoolPointerFlag) IsOptional() bool {
	return f.Optional
}
