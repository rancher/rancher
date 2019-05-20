package monitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prometheusReqTimeout = 30 * time.Second
)

var (
	defaultQueryDuring = "5m"
	defaultTo          = "now"
	defaultFrom        = "now-" + defaultQueryDuring
)

var (
	clusterResourceTypes = map[string]bool{
		ResourceCluster:          true,
		ResourceEtcd:             true,
		ResourceKubeComponent:    true,
		ResourceRancherComponent: true,
		ResourceNode:             true,
	}

	projectResourceTypes = map[string]bool{
		ResourceWorkload:  true,
		ResourcePod:       true,
		ResourceContainer: true,
	}
)

func newClusterGraphInputParser(input v3.QueryGraphInput) *clusterGraphInputParser {
	return &clusterGraphInputParser{
		Input: &input,
	}
}

type clusterGraphInputParser struct {
	Input        *v3.QueryGraphInput
	ClusterName  string
	ResourceType string
	Start        time.Time
	End          time.Time
	Step         time.Duration
	Conditions   []*types.QueryCondition
}

func (p *clusterGraphInputParser) parseClusterParams(userContext *config.UserContext, nodeLister v3.NodeLister) (err error) {
	p.Start, p.End, p.Step, err = parseTimeParams(p.Input.From, p.Input.To, p.Input.Interval)
	if err != nil {
		return err
	}

	return p.parseClusterMetricParams(userContext, nodeLister)
}

func (p *clusterGraphInputParser) parseFilter() error {
	if p.Input.Filters == nil {
		return fmt.Errorf("must have param filters")
	}

	p.ClusterName = p.Input.Filters["clusterId"]
	if p.ClusterName == "" {
		return fmt.Errorf(`param filters["clusterId"] is empty`)
	}

	p.ResourceType = p.Input.Filters["resourceType"]

	if p.ResourceType == "" {
		p.ResourceType = p.Input.Filters["displayResourceType"]
	}

	if p.ResourceType == "" {
		return fmt.Errorf(`param filters["resourceType"] is empty`)
	}

	if _, ok := clusterResourceTypes[p.ResourceType]; !ok {
		return fmt.Errorf(`invalid param filters["resourceType"`)
	}

	for name, value := range p.Input.Filters {
		p.Conditions = append(p.Conditions, types.NewConditionFromString(name, types.ModifierEQ, value))
	}

	return nil
}

func (p *clusterGraphInputParser) parseClusterMetricParams(userContext *config.UserContext, nodeLister v3.NodeLister) error {
	if p.Input.MetricParams == nil {
		p.Input.MetricParams = make(map[string]string)
		return nil
	}

	var ip string
	var err error
	if p.ResourceType == ResourceNode {
		instance := p.Input.MetricParams["instance"]
		if instance == "" {
			return fmt.Errorf(`param metricParams["instance"] is empty`)
		}
		ip, err = nodeName2InternalIP(nodeLister, p.ClusterName, instance)
		if err != nil {
			return err
		}
	}
	p.Input.MetricParams["instance"] = ip + ".*"
	return nil
}

type clusterAuthChecker struct {
	clusterID  string
	apiContext *types.APIContext
}

func newClusterAuthChecker(apiContext *types.APIContext, clusterID string) *clusterAuthChecker {
	return &clusterAuthChecker{
		clusterID:  clusterID,
		apiContext: apiContext,
	}
}

func (a *clusterAuthChecker) check() error {
	canGetCluster := func() error {
		cluster := map[string]interface{}{
			"id": a.clusterID,
		}

		clusterSchema := managementschema.Schemas.Schema(&managementschema.Version, mgmtclient.ClusterType)
		return a.apiContext.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "get", a.apiContext, cluster, clusterSchema)
	}

	return canGetCluster()
}

func newProjectGraphInputParser(input v3.QueryGraphInput) *projectGraphInputParser {
	return &projectGraphInputParser{
		Input: &input,
	}
}

type projectGraphInputParser struct {
	Input        *v3.QueryGraphInput
	ProjectID    string
	ClusterName  string
	ResourceType string
	Start        time.Time
	End          time.Time
	Step         time.Duration
	Conditions   []*types.QueryCondition
}

func (p *projectGraphInputParser) parseProjectParams(ctx context.Context, userContext *config.UserContext) (err error) {
	p.Start, p.End, p.Step, err = parseTimeParams(p.Input.From, p.Input.To, p.Input.Interval)
	if err != nil {
		return err
	}

	if p.Input.MetricParams == nil {
		p.Input.MetricParams = make(map[string]string)
	}

	return p.parseProjectMetricParams(ctx, userContext)
}

func (p *projectGraphInputParser) parseFilter() error {
	if p.Input.Filters == nil {
		return fmt.Errorf("must have param filters")
	}

	p.ProjectID = p.Input.Filters["projectId"]
	if p.ProjectID == "" {
		return fmt.Errorf(`param filters["projectId"] is empty`)
	}

	if p.ClusterName, _ = ref.Parse(p.ProjectID); p.ClusterName == "" {
		return fmt.Errorf(`invalid param filters["projectId"`)
	}

	p.ResourceType = p.Input.Filters["resourceType"]
	if p.ResourceType == "" {
		return fmt.Errorf(`param filters["resourceType"] is empty`)
	}

	if _, ok := projectResourceTypes[p.ResourceType]; !ok {
		return fmt.Errorf(`invalid param filters["resourceType"`)
	}

	for name, value := range p.Input.Filters {
		p.Conditions = append(p.Conditions, types.NewConditionFromString(name, types.ModifierEQ, value))
	}

	return nil
}

type projectAuthChecker struct {
	projectID   string
	input       *v3.QueryGraphInput
	userContext *config.UserContext
	apiContext  *types.APIContext
}

func newProjectAuthChecker(userContext *config.UserContext, apiContext *types.APIContext, input *v3.QueryGraphInput, projectID string) *projectAuthChecker {
	return &projectAuthChecker{
		projectID:   projectID,
		input:       input,
		userContext: userContext,
		apiContext:  apiContext,
	}
}

func (a *projectAuthChecker) check() error {
	canGetProject := func() error {
		project := map[string]interface{}{
			"id": a.projectID,
		}

		projectSchema := managementschema.Schemas.Schema(&managementschema.Version, mgmtclient.ProjectType)
		return a.apiContext.AccessControl.CanDo(v3.ProjectGroupVersionKind.Group, v3.ProjectResource.Name, "get", a.apiContext, project, projectSchema)
	}

	if err := canGetProject(); err != nil {
		return err
	}

	return a.parseNamespace()
}

func (a *projectAuthChecker) parseNamespace() error {
	if a.input.MetricParams["namespace"] != "" {
		if !a.isAuthorizeNamespace() {
			return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("couldn't query unauthorize namespace %s", a.input.MetricParams["namespace"]))
		}
		return nil
	}

	nss, err := a.getAuthroizeNamespace()
	if err != nil {
		return err
	}
	a.input.MetricParams["namespace"] = nss
	return nil
}

func (a *projectAuthChecker) isAuthorizeNamespace() bool {
	ns, err := a.userContext.Core.Namespaces(metav1.NamespaceAll).Get(a.input.MetricParams["namespace"], metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get namespace %s info failed, %v", a.input.MetricParams["namespace"], err)
		return false
	}
	return ns.Annotations[projectIDAnn] == a.projectID
}

func (a *projectAuthChecker) getAuthroizeNamespace() (string, error) {
	nss, err := a.userContext.Core.Namespaces(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list namespace failed, %v", err)
	}
	var authNs []string
	for _, v := range nss.Items {
		if v.Annotations[projectIDAnn] == a.projectID {
			authNs = append(authNs, v.Name)
		}
	}
	return strings.Join(authNs, "|"), nil
}

func getAuthToken(userContext *config.UserContext, appName, namespace string) (string, error) {
	sa, err := userContext.Core.ServiceAccounts(namespace).Get(appName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get service account %s:%s for monitor failed, %v", namespace, appName, err)
	}

	var secretName string
	if secretName = sa.Secrets[0].Name; secretName == "" {
		return "", fmt.Errorf("get secret from service account %s:%s for monitor failed, secret name is empty", namespace, appName)
	}

	secret, err := userContext.Core.Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get secret %s:%s for monitor failed, %v", namespace, secretName, err)
	}

	return string(secret.Data["token"]), nil
}

func (p *projectGraphInputParser) parseProjectMetricParams(ctx context.Context, userContext *config.UserContext) error {
	if p.Input.MetricParams == nil {
		p.Input.MetricParams = make(map[string]string)
		return nil
	}

	switch p.ResourceType {
	case ResourceWorkload:
		workloadName := p.Input.MetricParams["workloadName"]
		rcType, ns, name, err := parseWorkloadName(workloadName)
		if err != nil {
			return fmt.Errorf("get workload %s failed, %v", workloadName, err)
		}

		var podOwners []string
		if workloadName != "" {
			if rcType == workload.ReplicaSetType || rcType == workload.ReplicationControllerType || rcType == workload.DaemonSetType || rcType == workload.StatefulSetType || rcType == workload.JobType || rcType == workload.CronJobType {
				podOwners = []string{name}
			}

			if rcType == workload.DeploymentType {
				rcs, err := userContext.Apps.ReplicaSets(ns).List(metav1.ListOptions{})
				if err != nil {
					return fmt.Errorf("list replicasets failed, %v", err)
				}

				for _, rc := range rcs.Items {
					if len(rc.OwnerReferences) != 0 && strings.ToLower(rc.OwnerReferences[0].Kind) == workload.DeploymentType && rc.OwnerReferences[0].Name == name {
						podOwners = append(podOwners, rc.Name)
					}
				}
				rcType = workload.ReplicaSetType
			}

			var podNames []string
			pods, err := userContext.Core.Pods(ns).List(metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list pod failed, %v", err)
			}
			for _, pod := range pods.Items {
				if len(pod.OwnerReferences) != 0 {
					podRefName := pod.OwnerReferences[0].Name
					podRefKind := pod.OwnerReferences[0].Kind
					if contains(podRefName, podOwners...) && strings.ToLower(podRefKind) == rcType {
						podNames = append(podNames, pod.Name)
					}
				}
			}
			p.Input.MetricParams["podName"] = strings.Join(podNames, "|")
			p.Input.MetricParams["namespace"] = ns
		}
	case ResourcePod:
		podName := p.Input.MetricParams["podName"]
		if podName == "" {
			return fmt.Errorf(`param metricParams["podName"] is empty`)
		}
		ns, name := ref.Parse(podName)
		p.Input.MetricParams["namespace"] = ns
		p.Input.MetricParams["podName"] = name
	case ResourceContainer:
		podName := p.Input.MetricParams["podName"]
		if podName == "" {
			return fmt.Errorf(`param metricParams["podName"] is empty`)
		}
		ns, name := ref.Parse(podName)
		p.Input.MetricParams["namespace"] = ns
		p.Input.MetricParams["podName"] = name

		containerName := p.Input.MetricParams["containerName"]
		if containerName == "" {
			return fmt.Errorf(`param metricParams["containerName"] is empty`)
		}
	}
	return nil
}

func replaceParams(metricParams map[string]string, expr string) string {
	var replacer []string
	for k, v := range metricParams {
		replacer = append(replacer, "$"+k)
		replacer = append(replacer, v)
	}
	srp := strings.NewReplacer(replacer...)
	return srp.Replace(expr)
}

func parseTimeParams(from, to, interval string) (start, end time.Time, step time.Duration, err error) {
	if from == "" {
		from = defaultFrom
	}

	if to == "" {
		to = defaultTo
	}

	timeRange := NewTimeRange(from, to)
	start, err = timeRange.ParseFrom()
	if err != nil {
		err = fmt.Errorf("parse param from value %s failed, %v", from, err)
		return
	}

	end, err = timeRange.ParseTo()
	if err != nil {
		err = fmt.Errorf("parse param to value %s failed, %v", to, err)
		return
	}

	i, err := getIntervalFrom(interval, defaultMinInterval)
	if err != nil {
		err = fmt.Errorf("parse param interval value %s failed, %v", i, err)
		return
	}
	intervalCalculator := newIntervalCalculator(&IntervalOptions{MinInterval: i})
	calInterval := intervalCalculator.Calculate(timeRange, i)
	step = time.Duration(int64(calInterval.Value))
	return
}

func parseWorkloadName(id string) (typeName, namespace, name string, err error) {
	arr := strings.Split(id, ":")
	if len(arr) < 3 {
		return "", "", "", fmt.Errorf(`invalid param metricParams["workloadName"]: %s`, id)
	}
	return arr[0], arr[1], arr[2], nil
}

func contains(str string, arr ...string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}
func isInstanceGraph(graphType string) bool {
	return graphType == "singlestat"
}
