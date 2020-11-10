package systemtokens

type Interface interface {
	EnsureSystemToken(name, description, kind, username string, overrideTTL *int64, randomize bool) (string, error)
	DeleteToken(tokenName string) error
}
