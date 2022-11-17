package planner

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

func atMostThree(names []string) string {
	sort.Strings(names)
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ","), len(names)-3)
	}
	return strings.Join(names, ",")
}

func detailMessage(machines []string, messages map[string][]string) string {
	if len(machines) != 1 {
		return ""
	}
	message := messages[machines[0]]
	if len(message) != 0 {
		return fmt.Sprintf(": %s", strings.Join(message, ", "))
	}
	return ""
}

func removeReconciledCondition(machine *capi.Machine) *capi.Machine {
	if machine == nil || len(machine.Status.Conditions) == 0 {
		return machine
	}

	conds := make([]capi.Condition, 0, len(machine.Status.Conditions))
	for _, c := range machine.Status.Conditions {
		if string(c.Type) != string(rke2.Reconciled) {
			conds = append(conds, c)
		}
	}

	if len(conds) == len(machine.Status.Conditions) {
		return machine
	}

	machine = machine.DeepCopy()
	machine.SetConditions(conds)
	return machine
}

// ignoreErrors accepts two errors. If the err is type errIgnore, it will return (err, nil) if firstIgnoreErr is nil or (firstIgnoreErr, nil).
// Otherwise, it will simply return (firstIgnoreErr, err)
func ignoreErrors(firstIgnoreError error, err error) (error, error) {
	var errIgnore errIgnore
	if errors.As(err, &errIgnore) {
		if firstIgnoreError == nil {
			return err, nil
		}
		return firstIgnoreError, nil
	}
	return firstIgnoreError, err
}

// getControlPlaneJoinURL will return the first encountered join URL based on machine annotations for machines that are
// marked as control plane nodes
func getControlPlaneJoinURL(plan *plan.Plan) string {
	entries := collect(plan, isControlPlane)
	if len(entries) == 0 {
		return ""
	}

	return entries[0].Metadata.Annotations[rke2.JoinURLAnnotation]
}

// isUnavailable returns a boolean indicating whether the machine/node corresponding to the planEntry is available
// If the plan is not in sync or the machine is being drained, it will return true.
func isUnavailable(entry *planEntry) bool {
	return !entry.Plan.InSync || isInDrain(entry)
}

func isInDrain(entry *planEntry) bool {
	return entry.Metadata.Annotations[rke2.PreDrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.PostDrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.DrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.UnCordonAnnotation] != ""
}

// planAppliedButWaitingForProbes returns a boolean indicating whether a plan was successfully able to be applied, but
// the probes have not been successful. This indicates that while the overall plan hasn't completed yet, it's
// instructions have and can now be overridden if necessary without causing thrashing.
func planAppliedButWaitingForProbes(entry *planEntry) bool {
	return entry.Plan.AppliedPlan != nil && reflect.DeepEqual(entry.Plan.Plan, *entry.Plan.AppliedPlan) && !entry.Plan.Healthy
}

func calculateConcurrency(maxUnavailable string, entries []*planEntry, exclude roleFilter) (int, int, error) {
	var (
		count, unavailable int
	)

	for _, entry := range entries {
		if !exclude(entry) {
			count++
		}
		if entry.Plan != nil && isUnavailable(entry) {
			unavailable++
		}
	}

	num, err := strconv.Atoi(maxUnavailable)
	if err == nil {
		return num, unavailable, nil
	}

	if maxUnavailable == "" {
		return 1, unavailable, nil
	}

	percentage, err := strconv.ParseFloat(strings.TrimSuffix(maxUnavailable, "%"), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("concurrency must be a number or a percentage: %w", err)
	}

	max := float64(count) * (percentage / float64(100))
	return int(math.Ceil(max)), unavailable, nil
}

func minorPlanChangeDetected(old, new plan.NodePlan) bool {
	if !equality.Semantic.DeepEqual(old.Instructions, new.Instructions) ||
		!equality.Semantic.DeepEqual(old.PeriodicInstructions, new.PeriodicInstructions) ||
		!equality.Semantic.DeepEqual(old.Probes, new.Probes) ||
		old.Error != new.Error {
		return false
	}

	if len(old.Files) == 0 && len(new.Files) == 0 {
		// if the old plan had no files and no new files were found, there was no plan change detected
		return false
	}

	newFiles := make(map[string]plan.File)
	for _, newFile := range new.Files {
		newFiles[newFile.Path] = newFile
	}

	for _, oldFile := range old.Files {
		if newFile, ok := newFiles[oldFile.Path]; ok {
			if oldFile.Content == newFile.Content {
				// If the file already exists, we don't care if it is minor
				delete(newFiles, oldFile.Path)
			}
		} else {
			// the old file didn't exist in the new file map,
			// so check to see if the old file is major and if it is, this is not a minor change.
			if !oldFile.Minor {
				return false
			}
		}
	}

	if len(newFiles) > 0 {
		// If we still have new files in the list, check to see if any of them are major, and if they are, this is not a major change
		for _, newFile := range newFiles {
			// if we find a new major file, there is not a minor change
			if !newFile.Minor {
				return false
			}
		}
		// There were new files and all were not major
		return true
	}
	return false
}

func kubeletVersionUpToDate(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) bool {
	if controlPlane == nil || machine == nil || machine.Status.NodeInfo == nil || !controlPlane.Status.AgentConnected {
		// If any of controlPlane, machine, or machine.Status.NodeInfo are nil, then provisioning is still happening.
		// If controlPlane.Status.AgentConnected is false, then it cannot be reliably determined if the kubelet is up-to-date.
		// Return true so that provisioning is not slowed down.
		return true
	}

	kubeletVersion, err := semver.NewVersion(strings.TrimPrefix(machine.Status.NodeInfo.KubeletVersion, "v"))
	if err != nil {
		return false
	}

	kubernetesVersion, err := semver.NewVersion(strings.TrimPrefix(controlPlane.Spec.KubernetesVersion, "v"))
	if err != nil {
		return false
	}

	// Compare and ignore pre-release and build metadata
	return kubeletVersion.Major() == kubernetesVersion.Major() && kubeletVersion.Minor() == kubernetesVersion.Minor() && kubeletVersion.Patch() == kubernetesVersion.Patch()
}

// splitArgKeyVal takes a value and returns a pair (key, value) of the argument, or two empty strings if there was not
// a parsed key/val.
func splitArgKeyVal(val string, delim string) (string, string) {
	if splitSubArg := strings.SplitN(val, delim, 2); len(splitSubArg) == 2 {
		return splitSubArg[0], splitSubArg[1]
	}
	return "", ""
}

// getArgValue will search the passed in interface (arg) for a key that matches the searchArg. If a match is found, it
// returns the value of the argument, otherwise it returns an empty string.
func getArgValue(arg interface{}, searchArg string, delim string) string {
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) type of %v is %T", searchArg, delim, arg, arg)
	switch arg := arg.(type) {
	case []interface{}:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) encountered interface slice %v", searchArg, delim, arg)
		return getArgValue(convertInterfaceSliceToStringSlice(arg), searchArg, delim)
	case []string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string array: %v", searchArg, delim, arg)
		for _, v := range arg {
			argKey, argVal := splitArgKeyVal(v, delim)
			if argKey == searchArg {
				return argVal
			}
		}
	case string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string: %v", searchArg, delim, arg)
		argKey, argVal := splitArgKeyVal(arg, delim)
		if argKey == searchArg {
			return argVal
		}
	}
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) did not find searchArg in: %v", searchArg, delim, arg)
	return ""
}

// convertInterfaceSliceToStringSlice converts an input interface slice to a string slice by iterating through the
// interface slice and converting each entry to a string using Sprintf.
func convertInterfaceSliceToStringSlice(input []interface{}) []string {
	var stringArr []string
	for _, v := range input {
		stringArr = append(stringArr, fmt.Sprintf("%v", v))
	}
	return stringArr
}

// appendToInterface will return an interface that has the value appended to it. The interface returned will always be
// a slice of strings, and will convert a raw string to a slice of strings.
func appendToInterface(input interface{}, elem string) []string {
	switch input := input.(type) {
	case []interface{}:
		stringArr := convertInterfaceSliceToStringSlice(input)
		return appendToInterface(stringArr, elem)
	case []string:
		return append(input, elem)
	case string:
		return []string{input, elem}
	}
	return []string{elem}
}

// convertInterfaceToStringSlice converts an input interface to a string slice by determining its type and converting
// it accordingly. If it is not a known convertible type, an empty string slice is returned.
func convertInterfaceToStringSlice(input interface{}) []string {
	switch input := input.(type) {
	case []interface{}:
		return convertInterfaceSliceToStringSlice(input)
	case []string:
		return input
	case string:
		return []string{input}
	}
	return []string{}
}

// renderArgAndMount takes the value of the existing value of the argument and mount and renders an output argument and
// mount based on the value of the input interfaces. It will always return a set of slice of strings.
func renderArgAndMount(existingArg interface{}, existingMount interface{}, runtime string, defaultSecurePort string, defaultCertDir string) ([]string, []string) {
	retArg := convertInterfaceToStringSlice(existingArg)
	retMount := convertInterfaceToStringSlice(existingMount)
	renderedCertDir := fmt.Sprintf(defaultCertDir, runtime)
	// Set a default value for certDirArg and certDirMount (for the case where the user does not set these values)
	// If a user sets these values, we will set them to an empty string and check to make sure they are not empty
	// strings before adding them to the rendered arg/mount slices.
	certDirMount := fmt.Sprintf("%s:%s", renderedCertDir, renderedCertDir)
	certDirArg := fmt.Sprintf("%s=%s", CertDirArgument, renderedCertDir)
	securePortArg := fmt.Sprintf("%s=%s", SecurePortArgument, defaultSecurePort)
	if len(retArg) > 0 {
		tlsCF := getArgValue(retArg, TLSCertFileArgument, "=")
		if tlsCF == "" {
			// If the --tls-cert-file Argument was not set in the config for this component, we can look to see if
			// the --cert-dir was set. --tls-cert-file (if set) will take precedence over --tls-cert-file
			certDir := getArgValue(retArg, CertDirArgument, "=")
			if certDir != "" {
				// If --cert-dir was set, we use the --cert-dir that the user provided and should set certDirArg to ""
				// so that we don't append it.
				certDirArg = ""
				// Set certDirMount to an intelligently interpolated value based off of the custom certDir set by the
				// user.
				certDirMount = fmt.Sprintf("%s:%s", certDir, certDir)
			}
		} else {
			// If the --tls-cert-file argument was set by the user, we don't need to set --cert-dir, but still should
			// render a --cert-dir-mount that is based on the --tls-cert-file argument to map the files necessary
			// to the static pod (in the RKE2 case)
			certDirArg = ""
			dir := filepath.Dir(tlsCF)
			certDirMount = fmt.Sprintf("%s:%s", dir, dir)
		}
		sPA := getArgValue(retArg, SecurePortArgument, "=")
		if sPA != "" {
			// If the user set a custom --secure-port, set --secure-port to an empty string so we don't override
			// their custom value
			securePortArg = ""
		}
	}
	if certDirArg != "" {
		logrus.Debugf("renderArgAndMount adding %s to component arguments", certDirArg)
		retArg = appendToInterface(existingArg, certDirArg)
	}
	if securePortArg != "" {
		logrus.Debugf("renderArgAndMount adding %s to component arguments", securePortArg)
		retArg = appendToInterface(retArg, securePortArg)
	}
	if runtime == rke2.RuntimeRKE2 {
		// todo: make sure the certDirMount is not already set by the user to some custom value before we set it for the static pod extraMount
		logrus.Debugf("renderArgAndMount adding %s to component mounts", certDirMount)
		retMount = appendToInterface(existingMount, certDirMount)
	}
	return retArg, retMount
}

func PruneEmpty(config map[string]interface{}) {
	for k, v := range config {
		if v == nil {
			delete(config, k)
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				delete(config, k)
			}
		case []interface{}:
			if len(t) == 0 {
				delete(config, k)
			}
		case []string:
			if len(t) == 0 {
				delete(config, k)
			}
		}
	}
}

// getTaints returns a slice of taints for the machine in question
func getTaints(entry *planEntry) (result []corev1.Taint, _ error) {
	data := entry.Metadata.Annotations[rke2.TaintsAnnotation]
	if data != "" {
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return result, err
		}
	}

	if !isWorker(entry) {
		if isEtcd(entry) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/etcd",
				Effect: corev1.TaintEffectNoExecute,
			})
		}
		if isControlPlane(entry) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Effect: corev1.TaintEffectNoSchedule,
			})
		}
	}

	return
}

func getInstallerImage(controlPlane *rkev1.RKEControlPlane) string {
	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	installerImage := settings.SystemAgentInstallerImage.Get()
	installerImage = installerImage + runtime + ":" + strings.ReplaceAll(controlPlane.Spec.KubernetesVersion, "+", "-")
	return image.ResolveWithControlPlane(installerImage, controlPlane)
}

func isEtcd(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.EtcdRoleLabel] == "true"
}

func isInitNode(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.InitNodeLabel] == "true"
}

func isInitNodeOrDeleting(entry *planEntry) bool {
	return isInitNode(entry) || isDeleting(entry)
}

func IsEtcdOnlyInitNode(entry *planEntry) bool {
	return isInitNode(entry) && IsOnlyEtcd(entry)
}

func isNotInitNodeOrIsDeleting(entry *planEntry) bool {
	return !isInitNode(entry) || isDeleting(entry)
}

func isDeleting(entry *planEntry) bool {
	return entry.Machine.DeletionTimestamp != nil
}

// isFailed returns true if the provided entry machine.status.phase is failed
func isFailed(entry *planEntry) bool {
	return entry.Machine.Status.Phase == string(capi.MachinePhaseFailed)
}

// canBeInitNode returns true if the provided entry is an etcd node, is not deleting, is not failed, and has its infrastructure ready
// We should wait for the infrastructure condition to be marked as ready because we need the IP address(es) set prior to bootstrapping the node.
func canBeInitNode(entry *planEntry) bool {
	return isEtcd(entry) && !isDeleting(entry) && !isFailed(entry) && rke2.InfrastructureReady.IsTrue(entry.Machine)
}

func isControlPlane(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.ControlPlaneRoleLabel] == "true"
}

func isControlPlaneAndNotInitNode(entry *planEntry) bool {
	return isControlPlane(entry) && !isInitNode(entry)
}

func isControlPlaneEtcd(entry *planEntry) bool {
	return isControlPlane(entry) || isEtcd(entry)
}

func IsOnlyEtcd(entry *planEntry) bool {
	return isEtcd(entry) && !isControlPlane(entry)
}

func isOnlyControlPlane(entry *planEntry) bool {
	return !isEtcd(entry) && isControlPlane(entry)
}

func isWorker(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.WorkerRoleLabel] == "true"
}

func noRole(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && !isWorker(entry)
}

func anyRole(entry *planEntry) bool {
	return !noRole(entry)
}

func anyRoleWithoutWindows(entry *planEntry) bool {
	return !noRole(entry) && notWindows(entry)
}

func isOnlyWorker(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && isWorker(entry)
}

func notWindows(entry *planEntry) bool {
	return entry.Machine.Status.NodeInfo.OperatingSystem != windows
}

func isErrWaiting(err error) bool {
	var errWaiting ErrWaiting
	return errors.As(err, &errWaiting)
}
