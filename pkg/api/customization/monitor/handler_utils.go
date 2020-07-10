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
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
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

func newClusterGraphInputParser(input v3.QueryGraphInput) *clusterGraphInputParser {
	return &clusterGraphInputParser{
		Input: &input,
	}
}

type clusterGraphInputParser struct {
	Input       *v3.QueryGraphInput
	ClusterName string
	Start       time.Time
	End         time.Time
	Step        time.Duration
	Conditions  []*types.QueryCondition
}

func (p *clusterGraphInputParser) parse() (err error) {
	if p.Input.MetricParams == nil {
		p.Input.MetricParams = make(map[string]string)
	}

	p.Start, p.End, p.Step, err = parseTimeParams(p.Input.From, p.Input.To, p.Input.Interval)
	if err != nil {
		return err
	}

	return p.parseFilter()
}

func (p *clusterGraphInputParser) parseFilter() error {
	if p.Input.Filters == nil {
		return fmt.Errorf("must have clusterId filter")
	}

	p.ClusterName = p.Input.Filters["clusterId"]
	if p.ClusterName == "" {
		return fmt.Errorf("clusterId is empty")
	}

	for name, value := range p.Input.Filters {
		p.Conditions = append(p.Conditions, types.NewConditionFromString(name, types.ModifierEQ, value))
	}

	return nil
}

func newProjectGraphInputParser(input v3.QueryGraphInput) *projectGraphInputParser {
	return &projectGraphInputParser{
		Input: &input,
	}
}

type projectGraphInputParser struct {
	Input       *v3.QueryGraphInput
	ProjectID   string
	ClusterName string
	Start       time.Time
	End         time.Time
	Step        time.Duration
	Conditions  []*types.QueryCondition
}

func (p *projectGraphInputParser) parse() (err error) {
	if p.Input.MetricParams == nil {
		p.Input.MetricParams = make(map[string]string)
	}

	p.Start, p.End, p.Step, err = parseTimeParams(p.Input.From, p.Input.To, p.Input.Interval)
	if err != nil {
		return err
	}

	return p.parseFilter()
}

func (p *projectGraphInputParser) parseFilter() error {
	if p.Input.Filters == nil {
		return fmt.Errorf("must have projectId filter")
	}

	p.ProjectID = p.Input.Filters["projectId"]
	if p.ProjectID == "" {
		return fmt.Errorf("projectId is empty")
	}

	if p.ClusterName, _ = ref.Parse(p.ProjectID); p.ClusterName == "" {
		return fmt.Errorf("clusterName is empty")
	}

	for name, value := range p.Input.Filters {
		p.Conditions = append(p.Conditions, types.NewConditionFromString(name, types.ModifierEQ, value))
	}

	return nil
}

type authChecker struct {
	ProjectID          string
	Input              *v3.QueryGraphInput
	UserContext        *config.UserContext
	WorkloadController workload.CommonController
}

func newAuthChecker(ctx context.Context, userContext *config.UserContext, input *v3.QueryGraphInput, projectID string) *authChecker {
	return &authChecker{
		ProjectID:          projectID,
		Input:              input,
		UserContext:        userContext,
		WorkloadController: workload.NewWorkloadController(ctx, userContext.UserOnlyContext(), nil),
	}
}

func (a *authChecker) check() error {
	return a.parseNamespace()
}

func (a *authChecker) parseNamespace() error {
	if a.Input.MetricParams["namespace"] != "" {
		if !a.isAuthorizeNamespace() {
			return fmt.Errorf("could not query unauthorize namespace")
		}
		return nil
	}

	nss, err := a.getAuthroizeNamespace()
	if err != nil {
		return err
	}
	a.Input.MetricParams["namespace"] = nss
	return nil
}

func (a *authChecker) isAuthorizeNamespace() bool {
	ns, err := a.UserContext.Core.Namespaces(metav1.NamespaceAll).Get(a.Input.MetricParams["namespace"], metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get namespace %s info failed, %v", a.Input.MetricParams["namespace"], err)
		return false
	}
	return ns.Annotations[projectIDAnn] == a.ProjectID
}

func (a *authChecker) getAuthroizeNamespace() (string, error) {
	nss, err := a.UserContext.Core.Namespaces(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list namespace failed, %v", err)
	}
	var authNs []string
	for _, v := range nss.Items {
		if v.Annotations[projectIDAnn] == a.ProjectID {
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

func parseMetricParams(userContext *config.UserContext, nodeLister v3.NodeLister, resourceType, clusterName, projectName string, metricParams map[string]string) (map[string]string, error) {
	newMetricParams := make(map[string]string)
	for k, v := range metricParams {
		newMetricParams[k] = v
	}

	var ip string
	var err error
	switch resourceType {
	case ResourceNode:
		instance := newMetricParams["instance"]
		if instance == "" {
			return nil, fmt.Errorf("instance in metric params is empty")
		}
		ip, err = nodeName2InternalIP(nodeLister, clusterName, instance)
		if err != nil {
			return newMetricParams, err
		}

	case ResourceWorkload:
		workloadName := newMetricParams["workloadName"]
		rcType, ns, name, err := parseWorkloadName(workloadName)
		if err != nil {
			return newMetricParams, err
		}
		if !validateNS(newMetricParams, ns) {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can't access namespace %s from project %s", ns, projectName))
		}

		var podOwners []string
		if workloadName != "" {
			if rcType == workload.ReplicaSetType || rcType == workload.ReplicationControllerType || rcType == workload.DaemonSetType || rcType == workload.StatefulSetType || rcType == workload.JobType || rcType == workload.CronJobType {
				podOwners = []string{name}
			}

			if rcType == workload.DeploymentType {
				rcs, err := userContext.Apps.ReplicaSets(ns).List(metav1.ListOptions{})
				if err != nil {
					return newMetricParams, fmt.Errorf("list replicasets failed, %v", err)
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
				return nil, fmt.Errorf("list pod failed, %v", err)
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
			newMetricParams["podName"] = strings.Join(podNames, "|")
		}
	case ResourcePod:
		podName := newMetricParams["podName"]
		if podName == "" {
			return nil, fmt.Errorf("pod name is empty")
		}
		ns, name := ref.Parse(podName)
		if !validateNS(newMetricParams, ns) {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can't access namespace %s from project %s", ns, projectName))
		}
		newMetricParams["namespace"] = ns
		newMetricParams["podName"] = name
	case ResourceContainer:
		podName := newMetricParams["podName"]
		if podName == "" {
			return nil, fmt.Errorf("pod name is empty")
		}
		ns, name := ref.Parse(podName)
		if !validateNS(newMetricParams, ns) {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("can't access namespace %s from project %s", ns, projectName))
		}
		newMetricParams["namespace"] = ns
		newMetricParams["podName"] = name

		containerName := newMetricParams["containerName"]
		if containerName == "" {
			return nil, fmt.Errorf("container name is empty")
		}
	}
	newMetricParams["instance"] = ip + ".*"
	return newMetricParams, nil
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
		return "", "", "", fmt.Errorf("invalid workload name: %s", id)
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

func validateNS(params map[string]string, ns string) bool {
	value, ok := params["namespace"]
	if !ok {
		return false
	}
	nss := strings.Split(value, "|")
	for _, v := range nss {
		if v == ns {
			return true
		}
	}
	return false
}
