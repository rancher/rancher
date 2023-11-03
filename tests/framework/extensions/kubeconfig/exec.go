package kubeconfig

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8Scheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/cmd/cp"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	apiPath = "/api"
)

var podGroupVersion = corev1.SchemeGroupVersion.WithResource("pods").GroupVersion()

// LogStreamer is a struct that acts like io.Writer inorder to retrieve Stdout from a kubectl exec command in pod
type LogStreamer struct {
	b bytes.Buffer
}

// String stringer for the LogStreamer
func (l *LogStreamer) String() string {
	return l.b.String()
}

// Write is function that writes to the underlying bytes.Buffer
func (l *LogStreamer) Write(p []byte) (n int, err error) {
	a := strings.TrimSpace(string(p))
	l.b.WriteString(a)
	return len(p), nil
}

// KubectlExec is function that runs `kubectl exec` in a specified pod of a cluster. The function
// takes the kubeconfig in form of a restclient.Config object, the pod name, the namespace of the pod,
// and the command a user wants to run.
func KubectlExec(restConfig *restclient.Config, podName, namespace string, command []string) (*LogStreamer, error) {
	restConfig.ContentConfig.NegotiatedSerializer = serializer.NewCodecFactory(k8Scheme.Scheme)
	restConfig.ContentConfig.GroupVersion = &podGroupVersion
	restConfig.APIPath = apiPath

	restClient, err := restclient.RESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	req := restClient.Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	option := &corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	req.VersionedParams(
		option,
		k8Scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	logStreamer := &LogStreamer{}
	err = exec.StreamWithContext(context.TODO(), remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: logStreamer,
		Stderr: os.Stderr,
	})
	return logStreamer, err
}

// CopyFileFromPod is function that copies files from a pod. The parameter takes
// the kubeconfig in form of a restclient.Config object, the pod name, the namespace of the pod, the filename, and then
// the local destination (dest) where the file will be copied to.
func CopyFileFromPod(restConfig *restclient.Config, clientConfig clientcmd.ClientConfig, podName, namespace, filename, dest string) error {
	restConfig.ContentConfig.NegotiatedSerializer = serializer.NewCodecFactory(k8Scheme.Scheme)
	restConfig.ContentConfig.GroupVersion = &podGroupVersion
	restConfig.APIPath = apiPath

	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()
	copyOptions := cp.NewCopyOptions(ioStreams)

	newClientGett, err := NewRestGetter(restConfig, clientConfig)
	if err != nil {
		return err
	}

	newFactory := cmdutil.NewFactory(newClientGett)
	newCobra := &cobra.Command{}

	source := fmt.Sprintf("%s/%s:%s", namespace, podName, filename)
	if err := copyOptions.Complete(newFactory, newCobra, []string{source, dest}); err != nil {
		return err
	}

	err = copyOptions.Run()
	if err != nil {
		return err
	}
	return nil
}
