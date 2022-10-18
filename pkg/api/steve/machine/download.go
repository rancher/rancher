package machine

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rancher/apiserver/pkg/types"
)

func (s *sshClient) download(apiContext *types.APIRequest) error {
	machineInfo, err := s.getSSHKey(apiContext.Namespace, apiContext.Name)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	name := machineInfo.Driver.MachineName

	if err := addFile(zw, name+"/id_rsa", machineInfo.IDRSA); err != nil {
		return err
	}
	if err := addFile(zw, name+"/id_rsa.pub", machineInfo.IDRSAPub); err != nil {
		return err
	}
	machineConfigBytes, err := json.Marshal(machineInfo.Driver)
	if err != nil {
		return err
	}
	if err := addFile(zw, name+"/config.json", machineConfigBytes); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}

	apiContext.Response.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	apiContext.Response.Header().Set("Content-Type", "application/octet-stream")
	apiContext.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", name))
	apiContext.Response.Header().Set("Cache-Control", "private")
	apiContext.Response.Header().Set("Pragma", "private")
	apiContext.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
	apiContext.Response.WriteHeader(http.StatusOK)
	_, err = apiContext.Response.Write(buf.Bytes())
	return err
}

func addFile(zw *zip.Writer, name string, contents []byte) error {
	fh := &zip.FileHeader{
		Name: name,
	}
	fh.SetMode(0400)
	w, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	_, err = w.Write(contents)
	return err
}
