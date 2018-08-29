package model

type BuildInfo struct {
	TriggerType     string `json:"triggerType,omitempty"`
	TriggerUserName string `json:"triggerUserName,omitempty"`
	Commit          string `json:"commit,omitempty"`
	Event           string `json:"event,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Ref             string `json:"ref,omitempty"`
	RefSpec         string `json:"refSpec,omitempty"`
	HTMLLink        string `json:"htmlLink,omitempty"`
	Title           string `json:"title,omitempty"`
	Message         string `json:"message,omitempty"`
	Sender          string `json:"sender,omitempty"`
	Author          string `json:"author,omitempty"`
	AvatarURL       string `json:"avatarUrl,omitempty"`
	Email           string `json:"email,omitempty"`
}
