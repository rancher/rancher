package compression

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"

	"github.com/rancher/rancher/pkg/catalogv2/helm"
)

func CompressValuesYaml(input []byte) (string, error) {
	// If the object is empty, there's nothing to compress.
	if input == nil {
		return "", nil
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write(input)
	if err != nil {
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

func DecompressValuesYaml(input string) ([]byte, error) {
	// If the input is empty, there's nothing to decompress.
	if input == "" {
		return nil, nil
	}
	b := []byte(input)

	// If the input is not gzipped, we return it back.
	if !bytes.Equal(b[0:3], helm.MagicGzip) {
		return b, nil
	}
	gr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	output, err := ioutil.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return output, nil
}
