package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtclientv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config/dialer"
)

func NewProjectGraphHandler(dialerFactory dialer.Factory, clustermanager *clustermanager.Manager) *ProjectGraphHandler {
	return &ProjectGraphHandler{
		dialerFactory:  dialerFactory,
		clustermanager: clustermanager,
	}
}

type ProjectGraphHandler struct {
	dialerFactory  dialer.Factory
	clustermanager *clustermanager.Manager
}

func (h *ProjectGraphHandler) QuerySeriesAction(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var queryGraphInput v3.QueryGraphInput
	actionInput, err := parse.ReadBody(apiContext.Request)
	if err != nil {
		return err
	}

	if err = convert.ToObj(actionInput, &queryGraphInput); err != nil {
		return err
	}

	inputParser := newProjectGraphInputParser(queryGraphInput)
	if err = inputParser.parse(); err != nil {
		return err
	}

	clusterName := inputParser.ClusterName
	userContext, err := h.clustermanager.UserContext(clusterName)
	if err != nil {
		return fmt.Errorf("get usercontext failed, %v", err)
	}

	check := newAuthChecker(apiContext.Request.Context(), userContext, inputParser.Input, inputParser.ProjectID)
	if err = check.check(); err != nil {
		return err
	}

	prometheusName, prometheusNamespace := monitorutil.ClusterMonitoringInfo()
	token, err := getAuthToken(userContext, prometheusName, prometheusNamespace)
	if err != nil {
		return err
	}

	reqContext, cancel := context.WithTimeout(context.Background(), prometheusReqTimeout)
	defer cancel()

	svcName, svcNamespace, svcPort := monitorutil.ClusterPrometheusEndpoint()
	prometheusQuery, err := NewPrometheusQuery(reqContext, clusterName, token, svcNamespace, svcName, svcPort, h.dialerFactory)
	if err != nil {
		return err
	}

	var graphs []mgmtclientv3.ProjectMonitorGraph
	err = access.List(apiContext, apiContext.Version, mgmtclientv3.ProjectMonitorGraphType, &types.QueryOptions{Conditions: inputParser.Conditions}, &graphs)
	if err != nil {
		return err
	}

	mgmtClient := h.clustermanager.ScaledContext.Management
	var queries []*PrometheusQuery
	for _, graph := range graphs {
		g := graph
		_, projectName := ref.Parse(graph.ProjectID)
		refName := getRefferenceGraphName(projectName, graph.Name)
		monitorMetrics, err := graph2Metrics(userContext, mgmtClient, clusterName, g.ResourceType, refName, graph.MetricsSelector, graph.DetailsMetricsSelector, inputParser.Input.MetricParams, inputParser.Input.IsDetails)
		if err != nil {
			return err
		}

		queries = append(queries, metrics2PrometheusQuery(monitorMetrics, inputParser.Start, inputParser.End, inputParser.Step, isInstanceGraph(g.GraphType))...)
	}

	seriesSlice, err := prometheusQuery.Do(queries)
	if err != nil {
		return fmt.Errorf("query series failed, %v", err)
	}

	if seriesSlice == nil {
		apiContext.WriteResponse(http.StatusNoContent, nil)
		return nil
	}

	collection := v3.QueryProjectGraphOutput{Type: "collection"}
	for k, v := range seriesSlice {
		graphName, _, _ := parseID(k)
		queryGraph := v3.QueryProjectGraph{
			GraphName: graphName,
			Series:    parseResponse(v),
		}
		collection.Data = append(collection.Data, queryGraph)
	}

	res, err := json.Marshal(collection)
	if err != nil {
		return fmt.Errorf("marshal query series result failed, %v", err)
	}
	apiContext.Response.Write(res)
	return nil
}

func parseResponse(seriesSlice []*TimeSeries) []*v3.TimeSeries {
	var series []*v3.TimeSeries
	for _, v := range seriesSlice {
		series = append(series, &v3.TimeSeries{
			Name:   v.Name,
			Points: v.Points,
		})
	}
	return series
}
