package ext

import (
	"fmt"

	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	certutil "k8s.io/client-go/util/cert"
)

func certForCommonName(cname string) (dynamiccertificates.SNICertKeyContentProvider, error) {
	certPem, keyPem, err := certutil.GenerateSelfSignedCertKey(cname, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate self signed cert: %w", err)
	}

	content, err := dynamiccertificates.NewStaticSNICertKeyContent("self-signed-imperative", certPem, keyPem, cname)
	if err != nil {
		return nil, fmt.Errorf("failed to create static cert content: %w", err)
	}

	return content, nil
}
