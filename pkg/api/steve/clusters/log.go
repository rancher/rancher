package clusters

import (
	"bufio"
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
)

var (
	timeout int64 = 15 * 60
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
	Error:            onError,
}

func onError(rw http.ResponseWriter, _ *http.Request, code int, err error) {
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

type log struct {
	cg proxy.ClientGetter
}

func (l *log) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	err := l.printLog(resp, req)
	if err != nil {
		logrus.Infof("Error while handling cluster log: %v", err)
	}
}

func (l *log) printLog(resp http.ResponseWriter, req *http.Request) error {
	conn, err := upgrader.Upgrade(resp, req, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	apiRequest := types.GetAPIContext(req.Context())
	client, err := l.cg.AdminK8sInterface()
	if err != nil {
		return err
	}

	w, err := client.CoreV1().ConfigMaps(apiRequest.Name).Watch(req.Context(), metav1.ListOptions{
		TimeoutSeconds: &timeout,
		FieldSelector:  "metadata.name=provisioning-log",
	})
	if err != nil {
		return err
	}

	var (
		lastLine  = ""
		printLine = true
	)

	for event := range w.ResultChan() {
		switch event.Type {
		case watch.Added:
		case watch.Modified:
		case watch.Deleted:
			return nil
		default:
			continue
		}
		cm, ok := event.Object.(*corev1.ConfigMap)
		if !ok {
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(cm.Data["log"]))
		printed := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if !printLine && lastLine == line {
				printLine = true
				continue
			} else if printLine {
				if err := printMessage(line, conn); err != nil {
					return err
				}
				printed = true
				lastLine = line
			}
		}
		if !printed {
			scanner := bufio.NewScanner(strings.NewReader(cm.Data["log"]))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if err := printMessage(line, conn); err != nil {
					return err
				}
				lastLine = line
			}
		}
		printLine = false
	}

	return nil
}

func printMessage(msg string, conn *websocket.Conn) error {
	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(base64.StdEncoding.EncodeToString([]byte(msg)))); err != nil {
		return err
	}
	return writer.Close()
}

func (l *log) contextAndClient(req *http.Request) (context.Context, user.Info, kubernetes.Interface, error) {
	ctx := req.Context()
	client, err := l.cg.AdminK8sInterface()
	if err != nil {
		return ctx, nil, nil, err
	}

	user, ok := request.UserFrom(ctx)
	if !ok {
		return ctx, nil, nil, validation.Unauthorized
	}

	return ctx, user, client, nil
}
