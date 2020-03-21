package subscribe

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/norman/api/writer"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var upgrader = websocket.Upgrader{}

type Subscribe struct {
	ResourceTypes []string
	APIVersions   []string
	ProjectID     string `norman:"type=reference[/v3/schemas/project]"`
}

func Handler(apiContext *types.APIContext, _ types.RequestHandler) error {
	err := handler(apiContext)
	if err != nil {
		logrus.Errorf("Error during subscribe %v", err)
	}
	return err
}

func getMatchingSchemas(apiContext *types.APIContext) []*types.Schema {
	resourceTypes := apiContext.Request.URL.Query()["resourceTypes"]

	var schemas []*types.Schema
	for _, schema := range apiContext.Schemas.SchemasForVersion(*apiContext.Version) {
		if !matches(resourceTypes, schema.ID) {
			continue
		}
		if schema.Store != nil {
			schemas = append(schemas, schema)
		}
	}

	return schemas
}

type Expire interface {
	Expire(apiContext *types.APIContext)
}

func handler(apiContext *types.APIContext) error {
	schemas := getMatchingSchemas(apiContext)
	if len(schemas) == 0 {
		return httperror.NewAPIError(httperror.NotFound, "no resources types matched")
	}

	c, err := upgrader.Upgrade(apiContext.Response, apiContext.Request, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	cancelCtx, cancel := context.WithCancel(apiContext.Request.Context())
	readerGroup, ctx := errgroup.WithContext(cancelCtx)
	apiContext.Request = apiContext.Request.WithContext(ctx)

	go func() {
		for {
			if _, _, err := c.NextReader(); err != nil {
				cancel()
				c.Close()
				break
			}
		}
	}()

	events := make(chan map[string]interface{})
	for _, schema := range schemas {
		streamStore(ctx, readerGroup, apiContext, schema, events)
	}

	go func() {
		readerGroup.Wait()
		close(events)
	}()

	jsonWriter := writer.EncodingResponseWriter{
		ContentType: "application/json",
		Encoder:     types.JSONEncoder,
	}
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	done := false
	for !done {
		select {
		case item, ok := <-events:
			if !ok {
				done = true
				break
			}

			header := `{"name":"resource.change","data":`
			if item[".removed"] == true {
				header = `{"name":"resource.remove","data":`
			}
			schema := apiContext.Schemas.Schema(apiContext.Version, convert.ToString(item["type"]))
			if schema != nil {
				buffer := &bytes.Buffer{}

				apiContext.ExpireAccessControl(schema)
				if err := jsonWriter.VersionBody(apiContext, &schema.Version, buffer, item); err != nil {
					cancel()
					continue
				}

				if err := writeData(c, header, buffer.Bytes()); err != nil {
					cancel()
				}
			}
		case <-t.C:
			if err := writeData(c, `{"name":"ping","data":`, []byte("{}")); err != nil {
				cancel()
			}
		}
	}

	// no point in ever returning null because the connection is hijacked and we can't write it
	return nil
}

func writeData(c *websocket.Conn, header string, buf []byte) error {
	messageWriter, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err := messageWriter.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := messageWriter.Write(buf); err != nil {
		return err
	}
	if _, err := messageWriter.Write([]byte(`}`)); err != nil {
		return err
	}
	return messageWriter.Close()
}

func streamStore(ctx context.Context, eg *errgroup.Group, apiContext *types.APIContext, schema *types.Schema, result chan map[string]interface{}) {
	eg.Go(func() error {
		opts := parse.QueryOptions(apiContext, schema)
		events, err := schema.Store.Watch(apiContext, schema, &opts)
		if err != nil || events == nil {
			if err != nil {
				logrus.Errorf("failed on subscribe %s: %v", schema.ID, err)
			}
			return err
		}

		logrus.Tracef("watching %s", schema.ID)

		for e := range events {
			result <- e
		}

		return errors.New("disconnect")
	})
}

func matches(items []string, item string) bool {
	if len(items) == 0 {
		return true
	}
	return slice.ContainsString(items, item)
}
