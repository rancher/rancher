package model

//AuthConfig structure contains the AuthConfig definition
type AuthConfig struct {
	Provider     string        `json:"provider"`
	Enabled      bool          `json:"enabled"`
	GithubConfig *GithubConfig `json:"githubConfig,omitempty"`
	LocalConfig  *LocalConfig  `json:"localConfig,omitempty"`
}

//LocalConfig stores the local auth config
type LocalConfig struct {
}

//GithubConfig stores the github config read from JSON file
type GithubConfig struct {
	RedirectURL  string `json:"redirectUrl,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

func DefaultGithubConfig() AuthConfig {
	configObj := GithubConfig{}
	configObj.Hostname = ""
	configObj.Scheme = ""
	configObj.ClientID = "81a78dc20ac630bb93a6"
	configObj.ClientSecret = "2a821978299baf0480d9856368e7ab84b14bf86a"
	configObj.RedirectURL = "https://github.com/login/oauth/authorize?client_id=" + configObj.ClientID + "&scope=read:org"

	authConfig := AuthConfig{}
	authConfig.Provider = "github"
	authConfig.GithubConfig = &configObj

	return authConfig
}

func DefaultLocalConfig() AuthConfig {
	configObj := LocalConfig{}

	authConfig := AuthConfig{}
	authConfig.Provider = "local"
	authConfig.LocalConfig = &configObj
	return authConfig
}
