package client

const (
	WindowsSystemImagesType                    = "windowsSystemImages"
	WindowsSystemImagesFieldCalicoCNIBinaries  = "calicoCniBinaries"
	WindowsSystemImagesFieldCanalCNIBinaries   = "canalCniBinaries"
	WindowsSystemImagesFieldFlannelCNIBinaries = "flannelCniBinaries"
	WindowsSystemImagesFieldKubeletPause       = "kubeletPause"
	WindowsSystemImagesFieldKubernetesBinaries = "kubernetesBinaries"
	WindowsSystemImagesFieldNginxProxy         = "nginxProxy"
)

type WindowsSystemImages struct {
	CalicoCNIBinaries  string `json:"calicoCniBinaries,omitempty" yaml:"calicoCniBinaries,omitempty"`
	CanalCNIBinaries   string `json:"canalCniBinaries,omitempty" yaml:"canalCniBinaries,omitempty"`
	FlannelCNIBinaries string `json:"flannelCniBinaries,omitempty" yaml:"flannelCniBinaries,omitempty"`
	KubeletPause       string `json:"kubeletPause,omitempty" yaml:"kubeletPause,omitempty"`
	KubernetesBinaries string `json:"kubernetesBinaries,omitempty" yaml:"kubernetesBinaries,omitempty"`
	NginxProxy         string `json:"nginxProxy,omitempty" yaml:"nginxProxy,omitempty"`
}
