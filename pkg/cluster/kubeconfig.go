package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
)

type UserChan struct {
	UserId string
	WsId   string
}

type Handler struct {
	userMGR  user.Manager
	tokenMGR *tokens.Manager
}

var Done = make(chan UserChan)
var mapLock = sync.Mutex{}
var connections = map[string]*websocket.Conn{}

func KubeConfigTokenHander(ctx context.Context, mgmt *config.ScaledContext) *Handler {
	handler := &Handler{
		userMGR:  mgmt.UserManager,
		tokenMGR: tokens.NewManager(ctx, mgmt),
	}
	return handler
}

func (t *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	upgrader := websocket.Upgrader{}

	ts := req.Header.Get("Rancher_ConnToken")

	logrus.Infof("received token: %s", ts)

	wsConn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		logrus.Infof("error error %v", err)
		return
	}

	defer wsConn.Close()

	logrus.Infof("now starting for loop")

	endChan := make(chan bool)

	wsId := ""

	go func() {
		for {
			logrus.Infof("read channel")
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				logrus.Println("read err: msg", err, message)
				break
			}
			logrus.Infof("recv: %s", message)

			data := map[string]string{}

			json.Unmarshal(message, &data)

			wsId = data["wsId"]
			if wsId != "" {
				mapLock.Lock()
				connections[wsId] = wsConn
				mapLock.Unlock()
			}
		}

		logrus.Infof("got close? key %s", wsId)
		endChan <- true
	}()

	for {
		select {
		case x := <-Done:
			logrus.Infof("saml done response: %s", x.UserId)

			mapLock.Lock()
			wsConn := connections[x.WsId]
			mapLock.Unlock()

			err := wsConn.WriteMessage(websocket.TextMessage, []byte(x.UserId))
			if err != nil {
				logrus.Println("write:", err)
				return
			}

			logrus.Infof("sent correctly???")

		case y := <-endChan:
			logrus.Infof("received end!!!!! %v", y)
			return

		}
	}

	mapLock.Lock()
	delete(connections, wsId)
	logrus.Infof("coming out of for loop, will now close websocket here!")
}
