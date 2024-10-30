package roundtripper

import "net/http"

type UserAgent struct {
	UserAgent string
	Next      http.RoundTripper
}

func (u *UserAgent) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Set("User-Agent", u.UserAgent)
	return u.Next.RoundTrip(request)
}
