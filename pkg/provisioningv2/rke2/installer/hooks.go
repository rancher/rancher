package installer

// GetLinuxHook accepts a string indicating a hooks name
// and returns the value of that hook. If the given
// name does not belong to any defined hooks then an
// empty string is returned.
func GetLinuxHook(hookName string) string {
	switch hookName {
	case "disable-network-manager-cloud-setup":
		return disableNetworkManagerCloudSetupHook()
	}
	return ""
}

// GetWindowsHook accepts a string indicating a hooks name
// and returns the value of that hook. If the given
// name does not belong to any defined hooks then any
// empty string is returned.
func GetWindowsHook(hookName string) string {
	return "" // additional hooks for Windows systems can be defined here.
}

// disableNetworkManagerCloudSetupHook returns two commands
// which disable the nm-cloud-setup.timer & nm-cloud-setup.service
// processes if they are present on a linux system. These services
// interfere with CNI's on some OS's, such as RHEL. This
// hook will have no effect if the services are not present on the system.
func disableNetworkManagerCloudSetupHook() string {
	return `
sudo systemctl list-units --all | grep -Fq nm-cloud-setup.timer; if [ $? -eq 0 ]; then sudo systemctl disable nm-cloud-setup.timer; fi
sudo systemctl list-units --all | grep -Fq nm-cloud-setup.service; if [ $? -eq 0 ]; then sudo systemctl disable nm-cloud-setup.service; fi
`
}
