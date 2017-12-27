package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/management-api/api/setup"
	"github.com/rancher/management-api/controller/dynamicschema"
	"github.com/rancher/norman-rbac"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, management *config.ManagementContext) (http.Handler, error) {
	if err := setup.Schemas(ctx, management, management.Schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.AccessControl = rbac.NewAccessControl(management.RBAC)
	server.URLParser = urlParser(server.URLParser)

	if err := server.AddSchemas(management.Schemas); err != nil {
		return nil, err
	}

	dynamicschema.Register(management, server.Schemas)

	return server, nil
}

func urlParser(next parse.URLParser) parse.URLParser {
	return parse.URLParser(func(schemas *types.Schemas, url *url.URL) (parse.ParsedURL, error) {
		parsed, err := next(schemas, url)
		if err != nil {
			return parsed, err
		}

		if strings.HasPrefix(parsed.Type, client.ProjectType) && parsed.Link != "" {
			parts := strings.SplitN(parsed.Link, "/", 3)
			schema := schemas.Schema(&projectschema.Version, parts[0])
			if schema != nil {
				parsed.Query.Set(projectclient.SecretFieldProjectID, parsed.ID)
				parsed.SubContextPrefix = "/" + parsed.ID
				parsed.SubContext = map[string]string{
					"projects": parsed.ID,
				}
				parsed.Version = projectschema.Version.Path
				parsed.Type = schema.ID
				parsed.ID = ""
				parsed.Link = ""
				if len(parts) > 1 {
					parsed.ID = parts[1]
				}
				if len(parts) > 2 {
					parsed.Link = parts[2]
				}
			}
		}

		return parsed, nil
	})
}
