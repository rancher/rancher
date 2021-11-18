package tunnelserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/set"
	"github.com/rancher/rancher/pkg/peermanager"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/remotedialer"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewPeerManager(ctx context.Context, endpoints corecontrollers.EndpointsController, dialer *remotedialer.Server) (peermanager.PeerManager, error) {
	return startPeerManager(ctx, endpoints, dialer)
}

type peerManager struct {
	sync.Mutex
	leader        bool
	ready         bool
	token         string
	urlFormat     string
	server        *remotedialer.Server
	peers         map[string]bool
	endpointCache corecontrollers.EndpointsCache
	listeners     map[chan<- peermanager.Peers]bool
}

func getTokenFromToken(ctx context.Context, tokenBytes []byte) ([]byte, error) {
	// detect and handle 1.21+ token
	parts := strings.Split(string(tokenBytes), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token, not jwt format")
	}
	newBytes, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding service token: %w", err)
	}
	data := data.Object{}
	if err := json.Unmarshal(newBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal service token: %w", err)
	}
	ns := data.String("kubernetes.io", "namespace")
	name := data.String("kubernetes.io", "serviceaccount", "name")

	if name == "" || ns == "" {
		return tokenBytes, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	sa, err := client.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(sa.Secrets) == 0 {
		return nil, fmt.Errorf("no secret assigned to service account %s/%s", ns, name)
	}

	secret, err := client.CoreV1().Secrets(ns).Get(ctx, sa.Secrets[0].Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret.Data["token"], nil
}

func startPeerManager(ctx context.Context, endpoints corecontrollers.EndpointsController, server *remotedialer.Server) (peermanager.PeerManager, error) {
	tokenBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if os.IsNotExist(err) || settings.Namespace.Get() == "" || settings.PeerServices.Get() == "" {
		logrus.Infof("Running in single server mode, will not peer connections")
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	tokenBytes, err = getTokenFromToken(ctx, tokenBytes)
	if err != nil {
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
		token:         server.PeerToken,
		urlFormat:     "wss://%s/v3/connect",
		server:        server,
		peers:         map[string]bool{},
		endpointCache: endpoints.Cache(),
		listeners:     map[chan<- peermanager.Peers]bool{},
	}

	endpoints.OnChange(ctx, "peer-manager-controller", pm.syncService)
	return pm, nil
}

func (p *peerManager) syncService(key string, endpoint *v1.Endpoints) (*v1.Endpoints, error) {
	if endpoint == nil {
		return nil, nil
	}

	if endpoint.Namespace != settings.Namespace.Get() {
		return endpoint, nil
	}

	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return nil, nil
	}

	ns, name := parts[0], parts[1]
	if ns != settings.Namespace.Get() {
		return nil, nil
	}

	for _, svc := range strings.Split(settings.PeerServices.Get(), ",") {
		if name == strings.TrimSpace(svc) {
			if err := p.addRemovePeers(endpoint); err != nil {
				return endpoint, err
			}
			break
		}
	}

	return nil, nil
}

func (p *peerManager) addRemovePeers(endpoints *v1.Endpoints) error {
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
	return p.notify()
}

func (p *peerManager) getUserControllerPeers() (ready bool, result []string, err error) {
	userControllerService := settings.ControllerService.Get()
	if userControllerService == "" {
		for id := range p.peers {
			result = append(result, id)
		}
		return p.ready, result, nil
	}

	endpoints, err := p.endpointCache.Get(settings.Namespace.Get(), userControllerService)
	if err != nil {
		return false, nil, err
	}

	newSet := map[string]bool{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.IP == p.server.PeerID {
				ready = true
			} else {
				newSet[addr.IP] = true
			}
		}
	}

	for id := range newSet {
		result = append(result, id)
	}

	return
}

func (p *peerManager) notify() (err error) {
	peers := peermanager.Peers{
		Leader: p.leader,
		SelfID: p.server.PeerID,
	}

	peers.Ready, peers.IDs, err = p.getUserControllerPeers()
	if err != nil {
		return err
	}

	for c := range p.listeners {
		c <- peers
	}

	return nil
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
