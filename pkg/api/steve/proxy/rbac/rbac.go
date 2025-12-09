package rbac

import "net/http"

type (
	RequestProcessor interface {
		Process(r *http.Request) error
		Match(r *http.Request) bool
	}

	ResponseProcessor interface {
		Process(w http.ResponseWriter, r *http.Request)
		Match(r *http.Request) bool
	}
)

func RBAC(next http.Handler, apiServer http.Handler) http.Handler {
	requestProcessors := []RequestProcessor{
		&ProjectIDInjector{},
	}

	responseProcessors := []ResponseProcessor{
		&SchemaRbacProcessor{
			next,
			apiServer,
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		for _, p := range requestProcessors {
			if p.Match(r) {
				if err := p.Process(r); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
			}

		}

		for _, f := range responseProcessors {
			if f.Match(r) {
				f.Process(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
