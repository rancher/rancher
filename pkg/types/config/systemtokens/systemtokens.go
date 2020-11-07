package systemtokens

type Interface interface {
	EnsureSystemToken(name, description, kind, username string, overrideTTL *int64) (string, error)
}
