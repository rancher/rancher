package pipeline

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/sirupsen/logrus"
)

const (
	logSyncInterval  = 2 * time.Second
	writeWait        = time.Second
	longLogThreshold = 100000
	checkTailLength  = 1000
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

func (h *ExecutionHandler) handleLog(apiContext *types.APIContext) error {
	stageInput := apiContext.Request.URL.Query().Get("stage")
	stepInput := apiContext.Request.URL.Query().Get("step")
	stage, err := strconv.Atoi(stageInput)
	if err != nil {
		return err
	}
	step, err := strconv.Atoi(stepInput)
	if err != nil {
		return err
	}
	ns, name := ref.Parse(apiContext.ID)
	execution, err := h.PipelineExecutionLister.Get(ns, name)
	if err != nil {
		return err
	}
	clusterName, _ := ref.Parse(execution.Spec.ProjectName)
	userContext, err := h.ClusterManager.UserContext(clusterName)
	if err != nil {
		return err
	}

	pipelineEngine := engine.New(userContext, false)

	c, err := upgrader.Upgrade(apiContext.Response, apiContext.Request, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	cancelCtx, cancel := context.WithCancel(apiContext.Request.Context())
	apiContext.Request = apiContext.Request.WithContext(cancelCtx)

	go func() {
		for {
			if _, _, err := c.NextReader(); err != nil {
				cancel()
				c.Close()
				break
			}
		}
	}()

	prevLog := ""
	for range ticker.Context(cancelCtx, logSyncInterval) {
		execution, err = h.PipelineExecutionLister.Get(ns, name)
		if err != nil {
			logrus.Debugf("error in execution get: %v", err)
			if prevLog == "" {
				writeData(c, []byte("Log is unavailable."))
			}
			c.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(writeWait))
			return nil
		}
		log, err := pipelineEngine.GetStepLog(execution, stage, step)
		if err != nil {
			logrus.Debug(err)
			if prevLog == "" {
				writeData(c, []byte("Log is unavailable."))
			}
			c.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(writeWait))
			return nil
		}
		newLog := getNewLog(prevLog, log)
		prevLog = log
		if newLog != "" {
			if err := writeData(c, []byte(newLog)); err != nil {
				logrus.Debugf("error in writeData: %v", err)
				return nil
			}
		}
		if execution.Status.Stages[stage].Steps[step].Ended != "" {
			c.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(writeWait))
			return nil
		}
	}
	return nil
}

func writeData(c *websocket.Conn, buf []byte) error {
	messageWriter, err := c.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	defer messageWriter.Close()

	if _, err := messageWriter.Write(buf); err != nil {
		return err
	}

	return nil
}

func getNewLog(prevLog string, currLog string) string {
	if len(prevLog) < longLogThreshold {
		return strings.TrimPrefix(currLog, prevLog)
	}
	//long logs from Jenkins are trimmed so we use previous log tail to do comparison
	prevLogTail := prevLog[len(prevLog)-checkTailLength:]
	idx := strings.Index(currLog, prevLogTail)
	if idx >= 0 {
		return currLog[idx+checkTailLength:]
	}
	return currLog
}
