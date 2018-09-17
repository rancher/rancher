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
	"github.com/rancher/types/peermanager"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/net"
)

func NewPeerManager(context *config.ScaledContext, dialer *remotedialer.Server) (peermanager.PeerManager, error) {
	return startPeerManager(context, dialer)
}

type peerManager struct {
	sync.Mutex
	leader    bool
	ready     bool
	token     string
	urlFormat string
	server    *remotedialer.Server
	peers     map[string]bool
	listeners map[chan<- peermanager.Peers]bool
}

func startPeerManager(context *config.ScaledContext, server *remotedialer.Server) (peermanager.PeerManager, error) {
	tokenBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if os.IsNotExist(err) {
		logrus.Infof("Running in single server mode, will not peer connections")
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	ip, err := net.ChooseHostInterface()
	if err != nil {
		return nil, errors.Wrap(err, "choosing interface IP")
	}

	logrus.Infof("Running in clustered mode with ID %s, monitoring endpoint %s/%s", ip, settings.Namespace.Get(), settings.PeerServices.Get())

	server.PeerID = ip.String()
	server.PeerToken = string(tokenBytes)

	pm := &peerManager{
		token:     server.PeerToken,
		urlFormat: "wss://%s/v3/connect",
		server:    server,
		peers:     map[string]bool{},
		listeners: map[chan<- peermanager.Peers]bool{},
	}

	context.Core.Endpoints(settings.Namespace.Get()).AddHandler("peer-manager-controller", pm.syncService)
	return pm, nil
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
	ready := false
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP == p.server.PeerID {
				ready = true
			} else {
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
	p.ready = ready
	p.notify()
}

func (p *peerManager) notify() {
	peers := peermanager.Peers{
		Leader: p.leader,
		Ready:  p.ready,
		SelfID: p.server.PeerID,
	}

	for id := range p.peers {
		peers.IDs = append(peers.IDs, id)
	}

	for c := range p.listeners {
		c <- peers
	}
}

func (p *peerManager) AddListener(c chan<- peermanager.Peers) {
	p.Lock()
	defer p.Unlock()
	p.listeners[c] = true
}

func (p *peerManager) RemoveListener(c chan<- peermanager.Peers) {
	p.Lock()
	defer p.Unlock()
	delete(p.listeners, c)
	c <- peermanager.Peers{
		SelfID: p.server.PeerID,
		Leader: p.leader,
	}
}

func (p *peerManager) IsLeader() bool {
	p.Lock()
	defer p.Unlock()
	return p.leader
}

func (p *peerManager) Leader() {
	p.Lock()
	defer p.Unlock()

	if p.leader {
		return
	}

	p.leader = true
	p.notify()
}
