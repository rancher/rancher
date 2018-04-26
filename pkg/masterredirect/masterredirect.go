package masterredirect

import (
	"net/http"
	"sync"

	"github.com/rancher/rancher/pkg/clusterrouter/proxy"
)

type LeaderFunc func() (address string, leader bool)

func New(leader LeaderFunc, next http.Handler) http.Handler {
	return &server{
		leader: leader,
		next:   next,
	}
}

type server struct {
	sync.Mutex

	leader        LeaderFunc
	next          http.Handler
	simpleProxy   *proxy.SimpleProxy
	leaderAddress string
}

func (s *server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	addr, leader := s.leader()
	if leader {
		s.next.ServeHTTP(rw, req)
		return
	}

	proxy, err := s.proxy(addr)
	if err == nil {
		proxy.ServeHTTP(rw, req)
	} else {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
	}
}

func (s *server) proxy(addr string) (*proxy.SimpleProxy, error) {
	s.Lock()
	defer s.Unlock()

	if s.leaderAddress == addr {
		return s.simpleProxy, nil
	}

	p, err := proxy.NewSimpleInsecureProxy("https://" + addr)
	if err != nil {
		return nil, err
	}

	s.leaderAddress = addr
	s.simpleProxy = p
	return p, nil
}
