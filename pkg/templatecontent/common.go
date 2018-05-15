package templatecontent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetTemplateFromTag(tag string, templateContentClient v3.TemplateContentInterface) (string, error) {
	templateContent, err := templateContentClient.Get(tag, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	content, err := base64.StdEncoding.DecodeString(templateContent.Data)
	if err != nil {
		return "", err
	}
	reader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
