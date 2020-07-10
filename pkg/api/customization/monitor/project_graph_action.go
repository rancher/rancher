package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	monitorutil "github.com/rancher/rancher/pkg/monitoring"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	mgmtclientv3 "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewProjectGraphHandler(dialerFactory dialer.Factory, clustermanager *clustermanager.Manager) *ProjectGraphHandler {
	return &ProjectGraphHandler{
		dialerFactory:  dialerFactory,
		clustermanager: clustermanager,
		projectLister:  clustermanager.ScaledContext.Management.Projects(metav1.NamespaceAll).Controller().Lister(),
		appLister:      clustermanager.ScaledContext.Project.Apps(metav1.NamespaceAll).Controller().Lister(),
	}
}

type ProjectGraphHandler struct {
	dialerFactory  dialer.Factory
	clustermanager *clustermanager.Manager
	projectLister  v3.ProjectLister
	appLister      pv3.AppLister
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

	reqContext, cancel := context.WithTimeout(context.Background(), prometheusReqTimeout)
	defer cancel()

	var svcName, svcNamespace, svcPort, token string
	var queries []*PrometheusQuery
	prometheusName, prometheusNamespace := monitorutil.ClusterMonitoringInfo()
	token, err = getAuthToken(userContext, prometheusName, prometheusNamespace)
	if err != nil {
		return err
	}

	if inputParser.Input.Filters["resourceType"] == "istioproject" {
		if inputParser.Input.MetricParams["namespace"] == "" {
			return fmt.Errorf("no namespace found")
		}
		project, err := project.GetSystemProject(clusterName, h.projectLister)
		if err != nil {
			return err
		}
		app, err := h.appLister.Get(project.Name, monitorutil.IstioAppName)
		if err != nil {
			return err
		}
		svcName, svcNamespace, svcPort = monitorutil.IstioPrometheusEndpoint(app.Spec.Answers)

		mgmtClient := h.clustermanager.ScaledContext.Management
		istioGraphs, err := mgmtClient.ClusterMonitorGraphs(clusterName).List(metav1.ListOptions{LabelSelector: "component=istio,level=project"})
		if err != nil {
			return fmt.Errorf("list istio graph failed, %v", err)
		}
		for _, graph := range istioGraphs.Items {
			_, projectName := ref.Parse(inputParser.ProjectID)
			refName := getRefferenceGraphName(projectName, graph.Name)
			monitorMetrics, err := graph2Metrics(userContext, mgmtClient, clusterName, graph.Spec.ResourceType, refName, graph.Spec.MetricsSelector, graph.Spec.DetailsMetricsSelector, inputParser.Input.MetricParams, inputParser.Input.IsDetails)
			if err != nil {
				return err
			}

			queries = append(queries, metrics2PrometheusQuery(monitorMetrics, inputParser.Start, inputParser.End, inputParser.Step, isInstanceGraph(graph.Spec.GraphType))...)
		}
	} else {
		svcName, svcNamespace, svcPort = monitorutil.ClusterPrometheusEndpoint()

		var graphs []mgmtclientv3.ProjectMonitorGraph
		err = access.List(apiContext, apiContext.Version, mgmtclientv3.ProjectMonitorGraphType, &types.QueryOptions{Conditions: inputParser.Conditions}, &graphs)
		if err != nil {
			return err
		}

		mgmtClient := h.clustermanager.ScaledContext.Management
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
	}

	prometheusQuery, err := NewPrometheusQuery(reqContext, clusterName, token, svcNamespace, svcName, svcPort, h.dialerFactory, userContext)
	if err != nil {
		return err
	}
	seriesSlice, err := prometheusQuery.Do(queries)
	if err != nil {
		logrus.WithError(err).Warn("query series failed")
		return httperror.NewAPIError(httperror.ServerError, "Failed to obtain metrics. The metrics service may not be available.")
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
