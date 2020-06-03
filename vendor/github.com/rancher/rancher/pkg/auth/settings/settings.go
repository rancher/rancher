package settings

var (
	AuthUserInfoResyncCron    = newSetting("0 0 * * *")
	AuthUserSessionTTLMinutes = newSetting("960")  // 16 hours
	AuthUserInfoMaxAgeSeconds = newSetting("3600") // 1 hour
	FirstLogin                = newSetting("true")
)

type Setting interface {
	Get() string
	Set(val string) error
}

func newSetting(val string) Setting {
	return &setting{val}
}

type setting struct {
	val string
}

func (s *setting) Get() string {
	return s.val
}

func (s *setting) Set(val string) error {
	panic("not implemented")
}
