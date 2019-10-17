package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/rancher/rke/metadata"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	comments = `# If you intened to deploy Kubernetes in an air-gapped environment,
# please consult the documentation on how to configure custom RKE images.`
)

func ConfigCommand() cli.Command {
	return cli.Command{
		Name:   "config",
		Usage:  "Setup cluster configuration",
		Action: clusterConfig,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name,n",
				Usage: "Name of the configuration file",
				Value: pki.ClusterConfig,
			},
			cli.BoolFlag{
				Name:  "empty,e",
				Usage: "Generate Empty configuration file",
			},
			cli.BoolFlag{
				Name:  "print,p",
				Usage: "Print configuration",
			},
			cli.BoolFlag{
				Name:  "system-images,s",
				Usage: "Generate the default system images",
			},
			cli.BoolFlag{
				Name:  "list-version,l",
				Usage: "List the default kubernetes version",
			},
			cli.BoolFlag{
				Name:  "all,a",
				Usage: "Used with -s and -l, get all available versions",
			},
			cli.StringFlag{
				Name:  "version",
				Usage: "Generate the default system images for specific k8s versions",
			},
		},
	}
}

func getConfig(reader *bufio.Reader, text, def string) (string, error) {
	for {
		if def == "" {
			fmt.Printf("[+] %s [%s]: ", text, "none")
		} else {
			fmt.Printf("[+] %s [%s]: ", text, def)
		}
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)

		if input != "" {
			return input, nil
		}
		return def, nil
	}
}

func writeConfig(cluster *v3.RancherKubernetesEngineConfig, configFile string, print bool) error {
	yamlConfig, err := yaml.Marshal(*cluster)
	if err != nil {
		return err
	}
	logrus.Debugf("Deploying cluster configuration file: %s", configFile)

	configString := fmt.Sprintf("%s\n%s", comments, string(yamlConfig))
	if print {
		fmt.Printf("Configuration File: \n%s", configString)
		return nil
	}
	return ioutil.WriteFile(configFile, []byte(configString), 0640)
}

func clusterConfig(ctx *cli.Context) error {
	if metadata.K8sVersionToRKESystemImages == nil {
		err := metadata.InitMetadata(context.Background())
		if err != nil {
			return err
		}
	}

	if ctx.Bool("system-images") {
		return generateSystemImagesList(ctx.String("version"), ctx.Bool("all"))
	}

	if ctx.Bool("list-version") {
		if metadata.K8sVersionToRKESystemImages == nil {
			err := metadata.InitMetadata(context.Background())
			if err != nil {
				return err
			}
		}
		return generateK8sVersionList(ctx.Bool("all"))
	}

	configFile := ctx.String("name")
	engineConfig := v3.RancherKubernetesEngineConfig{}

	// Get engineConfig config from user
	reader := bufio.NewReader(os.Stdin)

	// Generate empty configuration file
	if ctx.Bool("empty") {
		engineConfig.Nodes = make([]v3.RKEConfigNode, 1)
		return writeConfig(&engineConfig, configFile, ctx.Bool("print"))
	}

	sshKeyPath, err := getConfig(reader, "Cluster Level SSH Private Key Path", "~/.ssh/id_rsa")
	if err != nil {
		return err
	}
	engineConfig.SSHKeyPath = sshKeyPath

	// Get number of hosts
	numberOfHostsString, err := getConfig(reader, "Number of Hosts", "1")
	if err != nil {
		return err
	}
	numberOfHostsInt, err := strconv.Atoi(numberOfHostsString)
	if err != nil {
		return err
	}

	// Get Hosts config
	engineConfig.Nodes = make([]v3.RKEConfigNode, 0)
	for i := 0; i < numberOfHostsInt; i++ {
		hostCfg, err := getHostConfig(reader, i, engineConfig.SSHKeyPath)
		if err != nil {
			return err
		}
		engineConfig.Nodes = append(engineConfig.Nodes, *hostCfg)
	}

	// Get Network config
	networkConfig, err := getNetworkConfig(reader)
	if err != nil {
		return err
	}
	engineConfig.Network = *networkConfig

	// Get Authentication Config
	authnConfig, err := getAuthnConfig(reader)
	if err != nil {
		return err
	}
	engineConfig.Authentication = *authnConfig

	// Get Authorization config
	authzConfig, err := getAuthzConfig(reader)
	if err != nil {
		return err
	}
	engineConfig.Authorization = *authzConfig

	// Get k8s/system images
	systemImages, err := getSystemImagesConfig(reader)
	if err != nil {
		return err
	}
	engineConfig.SystemImages = *systemImages

	// Get Services Config
	serviceConfig, err := getServiceConfig(reader)
	if err != nil {
		return err
	}
	engineConfig.Services = *serviceConfig

	//Get addon manifests
	addonsInclude, err := getAddonManifests(reader)
	if err != nil {
		return err
	}

	if len(addonsInclude) > 0 {
		engineConfig.AddonsInclude = append(engineConfig.AddonsInclude, addonsInclude...)
	}

	return writeConfig(&engineConfig, configFile, ctx.Bool("print"))
}

func getHostConfig(reader *bufio.Reader, index int, clusterSSHKeyPath string) (*v3.RKEConfigNode, error) {
	host := v3.RKEConfigNode{}

	address, err := getConfig(reader, fmt.Sprintf("SSH Address of host (%d)", index+1), "")
	if err != nil {
		return nil, err
	}
	host.Address = address

	port, err := getConfig(reader, fmt.Sprintf("SSH Port of host (%d)", index+1), cluster.DefaultSSHPort)
	if err != nil {
		return nil, err
	}
	host.Port = port

	sshKeyPath, err := getConfig(reader, fmt.Sprintf("SSH Private Key Path of host (%s)", address), "")
	if err != nil {
		return nil, err
	}
	if len(sshKeyPath) == 0 {
		fmt.Printf("[-] You have entered empty SSH key path, trying fetch from SSH key parameter\n")
		sshKey, err := getConfig(reader, fmt.Sprintf("SSH Private Key of host (%s)", address), "")
		if err != nil {
			return nil, err
		}
		if len(sshKey) == 0 {
			fmt.Printf("[-] You have entered empty SSH key, defaulting to cluster level SSH key: %s\n", clusterSSHKeyPath)
			host.SSHKeyPath = clusterSSHKeyPath
		} else {
			host.SSHKey = sshKey
		}
	} else {
		host.SSHKeyPath = sshKeyPath
	}

	sshUser, err := getConfig(reader, fmt.Sprintf("SSH User of host (%s)", address), "ubuntu")
	if err != nil {
		return nil, err
	}
	host.User = sshUser

	isControlHost, err := getConfig(reader, fmt.Sprintf("Is host (%s) a Control Plane host (y/n)?", address), "y")
	if err != nil {
		return nil, err
	}
	if isControlHost == "y" || isControlHost == "Y" {
		host.Role = append(host.Role, services.ControlRole)
	}

	isWorkerHost, err := getConfig(reader, fmt.Sprintf("Is host (%s) a Worker host (y/n)?", address), "n")
	if err != nil {
		return nil, err
	}
	if isWorkerHost == "y" || isWorkerHost == "Y" {
		host.Role = append(host.Role, services.WorkerRole)
	}

	isEtcdHost, err := getConfig(reader, fmt.Sprintf("Is host (%s) an etcd host (y/n)?", address), "n")
	if err != nil {
		return nil, err
	}
	if isEtcdHost == "y" || isEtcdHost == "Y" {
		host.Role = append(host.Role, services.ETCDRole)
	}

	hostnameOverride, err := getConfig(reader, fmt.Sprintf("Override Hostname of host (%s)", address), "")
	if err != nil {
		return nil, err
	}
	host.HostnameOverride = hostnameOverride

	internalAddress, err := getConfig(reader, fmt.Sprintf("Internal IP of host (%s)", address), "")
	if err != nil {
		return nil, err
	}
	host.InternalAddress = internalAddress

	dockerSocketPath, err := getConfig(reader, fmt.Sprintf("Docker socket path on host (%s)", address), cluster.DefaultDockerSockPath)
	if err != nil {
		return nil, err
	}
	host.DockerSocket = dockerSocketPath
	return &host, nil
}

func getSystemImagesConfig(reader *bufio.Reader) (*v3.RKESystemImages, error) {
	imageDefaults := metadata.K8sVersionToRKESystemImages[metadata.DefaultK8sVersion]

	kubeImage, err := getConfig(reader, "Kubernetes Docker image", imageDefaults.Kubernetes)
	if err != nil {
		return nil, err
	}

	systemImages, ok := metadata.K8sVersionToRKESystemImages[kubeImage]
	if ok {
		return &systemImages, nil
	}
	imageDefaults.Kubernetes = kubeImage
	return &imageDefaults, nil
}

func getServiceConfig(reader *bufio.Reader) (*v3.RKEConfigServices, error) {
	servicesConfig := v3.RKEConfigServices{}
	servicesConfig.Etcd = v3.ETCDService{}
	servicesConfig.KubeAPI = v3.KubeAPIService{}
	servicesConfig.KubeController = v3.KubeControllerService{}
	servicesConfig.Scheduler = v3.SchedulerService{}
	servicesConfig.Kubelet = v3.KubeletService{}
	servicesConfig.Kubeproxy = v3.KubeproxyService{}

	clusterDomain, err := getConfig(reader, "Cluster domain", cluster.DefaultClusterDomain)
	if err != nil {
		return nil, err
	}
	servicesConfig.Kubelet.ClusterDomain = clusterDomain

	serviceClusterIPRange, err := getConfig(reader, "Service Cluster IP Range", cluster.DefaultServiceClusterIPRange)
	if err != nil {
		return nil, err
	}
	servicesConfig.KubeAPI.ServiceClusterIPRange = serviceClusterIPRange
	servicesConfig.KubeController.ServiceClusterIPRange = serviceClusterIPRange

	podSecurityPolicy, err := getConfig(reader, "Enable PodSecurityPolicy", "n")
	if err != nil {
		return nil, err
	}
	if podSecurityPolicy == "y" || podSecurityPolicy == "Y" {
		servicesConfig.KubeAPI.PodSecurityPolicy = true
	} else {
		servicesConfig.KubeAPI.PodSecurityPolicy = false
	}

	clusterNetworkCidr, err := getConfig(reader, "Cluster Network CIDR", cluster.DefaultClusterCIDR)
	if err != nil {
		return nil, err
	}
	servicesConfig.KubeController.ClusterCIDR = clusterNetworkCidr

	clusterDNSServiceIP, err := getConfig(reader, "Cluster DNS Service IP", cluster.DefaultClusterDNSService)
	if err != nil {
		return nil, err
	}
	servicesConfig.Kubelet.ClusterDNSServer = clusterDNSServiceIP

	return &servicesConfig, nil
}

func getAuthnConfig(reader *bufio.Reader) (*v3.AuthnConfig, error) {
	authnConfig := v3.AuthnConfig{}

	authnType, err := getConfig(reader, "Authentication Strategy", cluster.DefaultAuthStrategy)
	if err != nil {
		return nil, err
	}
	authnConfig.Strategy = authnType
	return &authnConfig, nil
}

func getAuthzConfig(reader *bufio.Reader) (*v3.AuthzConfig, error) {
	authzConfig := v3.AuthzConfig{}
	authzMode, err := getConfig(reader, "Authorization Mode (rbac, none)", cluster.DefaultAuthorizationMode)
	if err != nil {
		return nil, err
	}
	authzConfig.Mode = authzMode
	return &authzConfig, nil
}

func getNetworkConfig(reader *bufio.Reader) (*v3.NetworkConfig, error) {
	networkConfig := v3.NetworkConfig{}

	networkPlugin, err := getConfig(reader, "Network Plugin Type (flannel, calico, weave, canal)", cluster.DefaultNetworkPlugin)
	if err != nil {
		return nil, err
	}
	networkConfig.Plugin = networkPlugin
	return &networkConfig, nil
}

func getAddonManifests(reader *bufio.Reader) ([]string, error) {
	var addonSlice []string
	var resume = true

	includeAddons, err := getConfig(reader, "Add addon manifest URLs or YAML files", "no")

	if err != nil {
		return nil, err
	}

	if strings.ContainsAny(includeAddons, "Yes YES Y yes y") {
		for resume {
			addonPath, err := getConfig(reader, "Enter the Path or URL for the manifest", "")
			if err != nil {
				return nil, err
			}

			addonSlice = append(addonSlice, addonPath)

			cont, err := getConfig(reader, "Add another addon", "no")
			if err != nil {
				return nil, err
			}

			if strings.ContainsAny(cont, "Yes y Y yes YES") {
				resume = true
			} else {
				resume = false
			}

		}
	}

	return addonSlice, nil
}

func generateK8sVersionList(all bool) error {
	if !all {
		fmt.Println(metadata.DefaultK8sVersion)
		return nil
	}

	for _, version := range metadata.K8sVersionsCurrent {
		if _, ok := metadata.K8sBadVersions[version]; !ok {
			fmt.Println(version)
		}
	}

	return nil
}

func generateSystemImagesList(version string, all bool) error {
	allVersions := []string{}
	currentVersionImages := make(map[string]v3.RKESystemImages)
	for _, version := range metadata.K8sVersionsCurrent {
		if _, ok := metadata.K8sBadVersions[version]; !ok {
			allVersions = append(allVersions, version)
			currentVersionImages[version] = metadata.K8sVersionToRKESystemImages[version]
		}
	}
	if all {
		for version, rkeSystemImages := range currentVersionImages {
			logrus.Infof("Generating images list for version [%s]:", version)
			uniqueImages := getUniqueSystemImageList(rkeSystemImages)
			for _, image := range uniqueImages {
				if image == "" {
					continue
				}
				fmt.Printf("%s\n", image)
			}
		}
		return nil
	}
	if len(version) == 0 {
		version = metadata.DefaultK8sVersion
	}
	rkeSystemImages := metadata.K8sVersionToRKESystemImages[version]
	if _, ok := metadata.K8sBadVersions[version]; ok {
		return fmt.Errorf("k8s version is not supported, supported versions are: %v", allVersions)
	}
	if rkeSystemImages == (v3.RKESystemImages{}) {
		return fmt.Errorf("k8s version is not supported, supported versions are: %v", allVersions)
	}
	logrus.Infof("Generating images list for version [%s]:", version)
	uniqueImages := getUniqueSystemImageList(rkeSystemImages)
	for _, image := range uniqueImages {
		if image == "" {
			continue
		}
		fmt.Printf("%s\n", image)
	}
	return nil
}

func getUniqueSystemImageList(rkeSystemImages v3.RKESystemImages) []string {
	// windows image not relevant for rke cli
	rkeSystemImages.WindowsPodInfraContainer = ""
	imagesReflect := reflect.ValueOf(rkeSystemImages)
	images := make([]string, imagesReflect.NumField())
	for i := 0; i < imagesReflect.NumField(); i++ {
		images[i] = imagesReflect.Field(i).Interface().(string)
	}
	return getUniqueSlice(images)
}

func getUniqueSlice(slice []string) []string {
	encountered := map[string]bool{}
	unqiue := []string{}

	for i := range slice {
		if encountered[slice[i]] {
			continue
		} else {
			encountered[slice[i]] = true
			unqiue = append(unqiue, slice[i])
		}
	}
	return unqiue
}
