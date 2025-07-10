package offline

import (
	"fmt"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"github.com/rancher/rancher/pkg/scc/util/log"
	"time"
)

func offlineValidatorContextLogger() log.StructuredLogger {
	logBuilder := &log.Builder{
		SubComponent: "offline-cert-validator",
	}
	return logBuilder.ToLogger()
}

// offlineCertError represents an issue encountered with offline certs; either a base error or wrapped error
type offlineCertError struct {
	Operation  string
	Details    string
	WrappedErr *error
}

func (e *offlineCertError) Error() string {
	msg := fmt.Sprintf("ValidateOfflineCertificateFailed during %s:", e.Operation)
	if e.WrappedErr != nil {
		msg += fmt.Sprintf(" (wrapped error: %v)", *e.WrappedErr)
	}
	if len(e.Details) > 0 {
		msg += fmt.Sprintf(" (details: %v)", e.Details)
	}
	return msg
}

func (e *offlineCertError) Unwrap() error {
	// If e.WrappedErr is nil, it means this offlineCertError is
	// the "base" or "original" error and doesn't wrap anything.
	// In this case, return nil as per the errors.Unwrap contract.
	if e.WrappedErr == nil {
		return nil
	}
	return *e.WrappedErr
}

type CertificateValidator struct {
	offlineCert        *registration.OfflineCertificate
	systemInfoExporter *systeminfo.InfoExporter
}

func New(offlineCert *registration.OfflineCertificate, systemInfoExporter *systeminfo.InfoExporter) *CertificateValidator {
	return &CertificateValidator{
		offlineCert:        offlineCert,
		systemInfoExporter: systemInfoExporter,
	}
}

func (cv *CertificateValidator) ValidateCertificate() error {
	// IsValid call sounds like it could be an overall "this is valid" status,
	// Ultimately it's a "was this signed by a source I trust" check (validates SHA and verifies PSS sig)
	certIsValid, validateErr := cv.offlineCert.IsValid()
	if validateErr != nil {
		return &offlineCertError{
			Operation:  "ValidateSignature",
			WrappedErr: &validateErr,
		}
	}

	if !certIsValid {
		return &offlineCertError{
			Operation: "ValidateSignature",
			Details:   "signature invalid",
		}
	}

	offlineCertExpiresAt, expiredErr := cv.offlineCert.ExpiresAt()
	if expiredErr != nil {
		return &offlineCertError{
			Operation:  "VerifyBeforeExpiresAt",
			WrappedErr: &expiredErr,
		}
	}
	if offlineCertExpiresAt.IsZero() || offlineCertExpiresAt.Before(time.Now()) {
		return &offlineCertError{
			Operation: "VerifyBeforeExpiresAt",
			Details:   "certificate has already expired",
		}
	}

	// Check if Offline Cert is using a wildcard UUID
	wildcardMatch, wildCardErr := cv.offlineCert.UUIDMatches("0x0")
	if wildCardErr != nil {
		offlineValidatorContextLogger().Warnf("failed to validate offline certificate: %v", wildCardErr)
	}
	if wildCardErr == nil && wildcardMatch {
		// This is an intentional early success for wildcards
		return nil
	}

	rancherUUID := cv.systemInfoExporter.RancherUuid()
	matchesRancherUUID, uidErr := cv.offlineCert.UUIDMatches(rancherUUID.String())
	if uidErr != nil {
		return &offlineCertError{
			Operation:  "VerifyInstallUUIDMatch",
			WrappedErr: &uidErr,
		}
	}

	if !matchesRancherUUID {
		return &offlineCertError{
			Operation: "VerifyInstallUUIDMatch",
			Details:   "certificate does not match Rancher UUID",
		}
	}

	return nil
}
