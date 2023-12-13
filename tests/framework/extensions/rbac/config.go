package rbac

const (
	ConfigurationFileKey = "rbacInput"
)

type Config struct {
	Role     string `json:"role" yaml:"role"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}
