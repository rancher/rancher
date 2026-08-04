package tokens

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteExpired removes every ext token whose TTL has passed and returns the
// number of tokens deleted. TTL-only: idle-expired tokens still within their
// TTL are rejected by the auth path but not deleted here.
//
// A failure to list tokens is returned as-is; the returned count is 0 in that
// case. Per-token delete failures do not stop iteration; each is wrapped with
// the token name and joined into the returned error, and the count reflects
// only tokens that were actually deleted (or already gone).
func (t *SystemStore) DeleteExpired() (int, error) {
	tokens, err := t.list(true, "", "", &metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("listing ext tokens: %w", err)
	}

	var (
		count int
		errs  []error
	)
	for _, tok := range tokens.Items {
		if !tok.GetIsExpired() {
			continue
		}
		if err := t.Delete(tok.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting %s: %w", tok.Name, err))
			continue
		}
		count++
	}

	return count, errors.Join(errs...)
}
