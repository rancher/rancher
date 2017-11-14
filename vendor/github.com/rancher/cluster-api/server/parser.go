package server

import (
	"net/url"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
)

var mgmtSchemas = types.NewSchemas().
	AddSchemas(managementSchema.Schemas)

func NewResolver(next parse.ResolverFunc) parse.ResolverFunc {
	return func(typeName string, apiContext *types.APIContext) error {
		return resolve(next, typeName, apiContext)
	}
}

func resolve(next parse.ResolverFunc, typeName string, apiContext *types.APIContext) error {
	//// Redirect /clusters/namespaces => /projects/namespaces
	//if apiContext.Version != nil && strings.HasPrefix(typeName, projectClient.NamespaceType) && apiContext.Version.Equals(&clusterSchema.Version) {
	//	apiContext.Schema = apiContext.Schemas.Schema(&projectSchema.Version, typeName)
	//	apiContext.Version = &projectSchema.Version
	//}
	//
	//if apiContext.Schema != nil {
	//	return nil
	//}

	return next(typeName, apiContext)
}

func URLParser(clusterName string, schemas *types.Schemas, url *url.URL) (parse.ParsedURL, error) {
	parsedURL, err := parse.DefaultURLParser(schemas, url)
	if err != nil {
		return parse.ParsedURL{}, err
	}

	if (parsedURL.Type == "clusters" || parsedURL.Type == "cluster") && parsedURL.ID == clusterName {
		parsedURL.Version = clusterSchema.Version.Path
	} else if (parsedURL.Type == "projects" || parsedURL.Type == "project") && parsedURL.ID != "" {
		parsedURL.Version = projectSchema.Version.Path
		parsedURL.SubContext = map[string]string{
			"projects": parsedURL.ID,
		}
	} else {
		return parse.ParsedURL{}, httperror.NewAPIError(httperror.NotFound, "failed to parse location")
	}

	parsedURL.SubContextPrefix = "/" + parsedURL.ID
	parsedURL.Type, parsedURL.ID, parsedURL.Link = threeSplit(parsedURL.Link)

	return parsedURL, nil
}

func threeSplit(link string) (string, string, string) {
	parts := strings.SplitN(link, "/", 3)

	switch len(parts) {
	case 2:
		return parts[0], parts[1], ""
	case 3:
		return parts[0], parts[1], parts[3]
	default:
		return parts[0], "", ""
	}
}
