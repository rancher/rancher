package tunnelserver

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/rancher/pkg/remotedialer"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/net"
)

type peerManager struct {
	sync.Mutex
	token     string
	urlFormat string
	server    *remotedialer.Server
	peers     map[string]bool
}

func startPeerManager(context *config.ScaledContext, server *remotedialer.Server) error {
	tokenBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if os.IsNotExist(err) {
		logrus.Infof("Running in single server mode, will not peer connections")
		return nil
	} else if err != nil {
		return err
	}

	ip, err := net.ChooseHostInterface()
	if err != nil {
		return errors.Wrap(err, "choosing interface IP")
	}

	logrus.Infof("Running in clustered mode with ID %s, monitoring endpoint %s/%s", ip, settings.Namespace.Get(), settings.PeerServices.Get())

	server.PeerID = ip.String()
	server.PeerToken = string(tokenBytes)

	pm := &peerManager{
		token:     server.PeerToken,
		urlFormat: "wss://%s/v3/connect",
		server:    server,
		peers:     map[string]bool{},
	}

	context.Core.Endpoints(settings.Namespace.Get()).AddHandler("peer-manager-controller", pm.syncService)
	return nil
}

func (p *peerManager) syncService(key string, endpoint *v1.Endpoints) error {
	if endpoint == nil {
		return nil
	}

	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return nil
	}

	ns, name := parts[0], parts[1]
	if ns != settings.Namespace.Get() {
		return nil
	}

	for _, svc := range strings.Split(settings.PeerServices.Get(), ",") {
		if name == strings.TrimSpace(svc) {
			p.addRemovePeers(endpoint)
			break
		}
	}

	return nil
}

func (p *peerManager) addRemovePeers(endpoints *v1.Endpoints) {
	p.Lock()
	defer p.Unlock()

	newSet := map[string]bool{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP != p.server.PeerID {
				newSet[addr.IP] = true
			}
		}
	}

	toCreate, toDelete, _ := set.Diff(newSet, p.peers)
	for _, ip := range toCreate {
		p.server.AddPeer(fmt.Sprintf(p.urlFormat, ip), ip, p.token)
	}
	for _, ip := range toDelete {
		p.server.RemovePeer(ip)
	}
	p.peers = newSet
}
