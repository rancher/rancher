package offline

import (
	"errors"
	"fmt"
	"github.com/SUSE/connect-ng/pkg/registration"
	"github.com/rancher/rancher/pkg/scc/systeminfo"
	"time"
)

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
	certIsValid, validateErr := cv.offlineCert.IsValid()
	if validateErr != nil {
		return validateErr
	}

	if !certIsValid {
		return errors.New("certificate is not valid")
	}

	// TODO: does offline need users to define RegCode for this?
	// matchesRegCode, regCodeErr := cv.offlineCert.RegcodeMatches("")

	rancherUUID := cv.systemInfoExporter.RancherUuid()
	matchesRancherUUID, uidErr := cv.offlineCert.UUIDMatches(rancherUUID.String())
	if uidErr != nil {
		return fmt.Errorf("failed to validate Rancher UUID: %v", uidErr)
	}

	if !matchesRancherUUID {
		return errors.New("current rancher UUID does not match the offline cert's rancher UUID")
	}

	offlineCertExpiresAt, expiredErr := cv.offlineCert.ExpiresAt()
	if expiredErr != nil {
		return fmt.Errorf("failed to validate offline certificate: %v", expiredErr)
	}
	if offlineCertExpiresAt.IsZero() || offlineCertExpiresAt.Before(time.Now()) {
		return fmt.Errorf("certificate has expired")
	}
	prod, _, _ := cv.systemInfoExporter.GetProductIdentifier()
	matchesProductClass, err := cv.offlineCert.ProductClassIncluded(prod)
	if err != nil {
		return fmt.Errorf("failed to validate product class: %v", err)
	}

	if !matchesProductClass {
		return fmt.Errorf("product class does not match the offline cert's product class")
	}

	return nil
}
