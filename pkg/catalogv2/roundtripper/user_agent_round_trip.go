package roundtripper

import (
	"fmt"
	"net/http"

	"github.com/rancher/rancher/pkg/settings"
)

type UserAgent struct {
	UserAgent string
	Next      http.RoundTripper
}

func (u *UserAgent) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Set("User-Agent", u.UserAgent)
	return u.Next.RoundTrip(request)
}

// BuildUserAgent constructs a standardized User-Agent string for Helm repository access
func BuildUserAgent(protocol, context string) string {
	return fmt.Sprintf("%s/rancher/%s/%s %s",
		protocol,
		settings.ServerVersionType.Get(),
		settings.ServerVersion.Get(),
		context)
}
