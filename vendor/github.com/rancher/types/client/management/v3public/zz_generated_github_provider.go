package client

const (
	GithubProviderType                 = "githubProvider"
	GithubProviderFieldAnnotations     = "annotations"
	GithubProviderFieldCreated         = "created"
	GithubProviderFieldCreatorID       = "creatorId"
	GithubProviderFieldLabels          = "labels"
	GithubProviderFieldName            = "name"
	GithubProviderFieldOwnerReferences = "ownerReferences"
	GithubProviderFieldRemoved         = "removed"
	GithubProviderFieldType            = "type"
	GithubProviderFieldUuid            = "uuid"
)

type GithubProvider struct {
	Annotations     map[string]string `json:"annotations,omitempty"`
	Created         string            `json:"created,omitempty"`
	CreatorID       string            `json:"creatorId,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Name            string            `json:"name,omitempty"`
	OwnerReferences []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed         string            `json:"removed,omitempty"`
	Type            string            `json:"type,omitempty"`
	Uuid            string            `json:"uuid,omitempty"`
}
