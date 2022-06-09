package clean

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

const (
	DockerPipe = "\\\\.\\pipe\\docker_engine"
	WinsPipe   = "\\\\.\\pipe\\rancher_wins"
	HostMount  = "c:\\host\\"
)

func Run(ctx context.Context, args []string) error {
	if len(args) > 3 {
		fmt.Println(usage())
		return nil
	}

	if len(args) == 3 {
		switch args[2] {
		case "job":
			return job(ctx)
		case "node":
			return node(ctx)
		case "script", "scripts":
			fmt.Print(script())
			return nil
		case "help":
			fmt.Println(usage())
			return nil
		default:
			fmt.Println(usage())
			return nil
		}
	}

	fmt.Println(usage())
	return nil
}

func job(ctx context.Context) error {
	logrus.Infof("Starting clean container job: %s", NodeCleanupContainerName)

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containerList {
		for _, n := range c.Names {
			if n == "/"+NodeCleanupContainerName {
				logrus.Infof("container named %s already exists, exiting.", NodeCleanupContainerName)
				return nil
			}
		}
	}

	binds := []string{
		fmt.Sprintf("%s:%s", DockerPipe, DockerPipe),
		fmt.Sprintf("%s:%s", WinsPipe, WinsPipe),
		fmt.Sprintf("%s:%s", "c:\\", HostMount),
	}

	container, err := c.ContainerCreate(ctx, &container.Config{
		Image: getAgentImage(),
		Env: []string{
			fmt.Sprintf("%s=%s", AgentImage, getAgentImage()),
			fmt.Sprintf("%s=%s", PrefixPath, os.Getenv(PrefixPath)),
			fmt.Sprintf("%s=%s", WindowsPrefixPath, os.Getenv(WindowsPrefixPath)),
		},
		Cmd: []string{"--", "agent", "clean", "node"},
	}, &container.HostConfig{
		Binds: binds,
	}, nil, nil, NodeCleanupContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, container.ID, types.ContainerStartOptions{})
}

func node(ctx context.Context) error {
	logrus.Info("Cleaning up node...")

	c, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.FromEnv)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := waitForK8sPods(ctx, c); err != nil {
		return fmt.Errorf("error waiting for k8s pods to be removed: %s", err)
	}

	return spawnCleanup()
}

func spawnCleanup() error {
	if err := writeScript(); err != nil {
		return err
	}
	winsArgs := createWinsArgs("Spawn")
	output, err := exec.Command("wins.exe", winsArgs...).Output()
	if err != nil {
		logrus.Infof(string(output))
		return err
	}
	return nil
}

func writeScript() error {
	// add a null file to the container for wins to find and make a hash
	psPath := getPowershellPath()
	if !fileExists(psPath) {
		psHostPath := strings.Replace(psPath, "c:\\", HostMount, 1)

		src, err := os.Open(psHostPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(psPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	} else {
		logrus.Infof("powershell.exe already exists: %s", psPath)
	}

	// write one to the host for wins cli to call
	scriptBytes := []byte(script())
	hostScriptPath := strings.Replace(getScriptPath(), "c:\\", HostMount, 1)
	if !fileExists(hostScriptPath) {
		logrus.Infof("writing file to host: %s", hostScriptPath)
		if err := ioutil.WriteFile(hostScriptPath, scriptBytes, 0777); err != nil {
			return fmt.Errorf("error writing the cleanup script to the host: %s", err)
		}
	} else {
		logrus.Infof("cleanup script already exists on host: %s", hostScriptPath)
	}

	return nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func getAgentImage() string {
	agentImage := os.Getenv(AgentImage)
	if agentImage == "" {
		agentImage = "rancher/rancher-agent:master"
	}
	return agentImage
}

func waitForK8sPods(ctx context.Context, c *client.Client) error {
	// wait for up to 5min for k8s pods to be dropped
	for i := 0; i < 30; i++ {
		logrus.Infof("checking for pods %d out of 30 times", i)
		containerList, err := c.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			return err
		}

		hasPods := false
		for _, c := range containerList {
			for _, n := range c.Names {
				if strings.HasPrefix(n, "/k8s_") {
					hasPods = true
					continue
				}
			}
			if hasPods {
				continue //break out if you already found one
			}
		}

		if hasPods {
			logrus.Info("pods found, waiting 10s and trying again")
			time.Sleep(10 * time.Second)
			continue
		}

		logrus.Info("all pods cleaned, continuing on to more rke cleanup")
		return nil
	}

	return nil
}

func createWinsArgs(tasks ...string) []string {
	args := fmt.Sprintf("-File %s", getScriptPath())
	if len(tasks) > 0 {
		args = fmt.Sprintf("%s -Tasks %s", args, strings.Join(tasks, ","))
	}

	path := getPowershellPath()
	logrus.Infof("path: %s, args: %s", path, args)

	return []string{
		"cli", "prc", "run",
		"--path", path,
		"--args", args,
	}
}

func getPrefixPath() string {
	prefix := os.Getenv(WindowsPrefixPath)
	if prefix == "" {
		prefix = "c:\\"
	}
	return prefix
}

func getScriptPath() string {
	return filepath.Join(getPrefixPath(), "etc", "rancher", "cleanup.ps1")
}

func getPowershellPath() string {
	return filepath.Join(getPrefixPath(), "etc", "rancher", "powershell.exe")
}

func usage() string {
	return `agent clean usage for windows:
Windows is hard to cleanup from a container and it is recommended to clean a node that you pull the included PowerShell
script out of the container and run it locally as administrator. Read the top of the file for usage of the -Tasks argument
if you only want to clean pieces of the node.

You can clean an entire node with this one line command:
docker run rancher/rancher-agent:master -- agent clean script > cleanup.ps1; ./cleanup.ps1

Note: If your cluster was created with a prefixPath use the env param -e WINDOWS_PREFIX_PATH=c:\my\prefix

commands:
	script - prints a PowerShell script you can run to clean the node
	help - print this help message

other commands for automation
	node - cleans the entire node, requires volume and named pipes mounted and is best run via automation
	job - used by the k8s batch job to start the ` + NodeCleanupContainerName + ` container to watch kubelet cleanup and wait to clean the node
`
}

func script() string {
	return `#Requires -RunAsAdministrator
<#
.SYNOPSIS 
    Cleans Rancher managed Windows Worker Nodes. Backup your data. Use at your own risk.
.DESCRIPTION 
    Run the script to clean the windows host of all Rancher related data (kubernetes, docker, network) 
.NOTES
    This script needs to be run with Elevated permissions to allow for the complete collection of information.
    Backup your data.
    Use at your own risk.
.EXAMPLE 
    cleanup.ps1    
    Clean the windows host of all Rancher related data (kubernetes, docker, network).

    cleanup.ps1 -Tasks Docker
    Cleans the windows host of all Rancher docker related data.

    cleanup.ps1 -Tasks Docker,Network
    Cleans the windows host of all Rancher docker and network related data.
#>
[CmdletBinding()]
param (
    [Parameter()]
    [ValidateSet("Docker", "Kubernetes", "Firewall", "Rancher", "Network", "Paths", "Spawn", "Logs")]
    [string[]]
    $Tasks = ("Docker", "Kubernetes", "Rancher", "Firewall", , "Network", "Paths")
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Test-Command($cmdname) {
    return [bool](Get-Command -Name $cmdname -ErrorAction SilentlyContinue)
}

# Write-EventLog for when run via automation, Write-Host when running by hand.
function Log-Info {
	Write-EventLog -LogName "Application" -Source ` + NodeCleanupContainerName + ` -EventID 999 -EntryType Information -Message $($args -join " ") 
    Write-Host -NoNewline -ForegroundColor Blue "INFO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Warn {
	Write-EventLog -LogName "Application" -Source ` + NodeCleanupContainerName + ` -EventID 999 -EntryType Warning -Message $($args -join " ")
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Error {
	Write-EventLog -LogName "Application" -Source ` + NodeCleanupContainerName + ` -EventID 999 -EntryType Error -Message $($args -join " ")
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Fatal {
	Write-EventLog -LogName "Application" -Source ` + NodeCleanupContainerName + ` -EventID 999 -EntryType Error -Message $($args -join " ")
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
    exit 255
}

function Get-VmComputeNativeMethods() {
    $ret = 'VmCompute.PrivatePInvoke.NativeMethods' -as [type]
    if (-not $ret) {
        $signature = @'
[DllImport("vmcompute.dll")]
public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@
        $ret = Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name "NativeMethods" -PassThru
    }
    return $ret
}

function Invoke-HNSRequest {
    param
    (
        [ValidateSet('GET', 'DELETE')]
        [parameter(Mandatory = $true)] [string] $Method,
        [ValidateSet('networks', 'endpoints', 'activities', 'policylists', 'endpointstats', 'plugins')]
        [parameter(Mandatory = $true)] [string] $Type,
        [parameter(Mandatory = $false)] [string] $Action,
        [parameter(Mandatory = $false)] [string] $Data = "",
        [parameter(Mandatory = $false)] [Guid] $Id = [Guid]::Empty
    )
    $hnsPath = "/$Type"
    if ($id -ne [Guid]::Empty) {
        $hnsPath += "/$id"
    }
    if ($Action) {
        $hnsPath += "/$Action"
    }
    $response = ""
    $hnsApi = Get-VmComputeNativeMethods
    $hnsApi::HNSCall($Method, $hnsPath, "$Data", [ref]$response)
    $output = @()
    if ($response) {
        try {
            $output = ($response | ConvertFrom-Json)
            if ($output.Error) {
                Log-Error $output;
            }
            else {
                $output = $output.Output;
            }
        }
        catch {
            Log-Error $_.Exception.Message
        }
    }
    return $output;
}

function Remove-DockerContainers {
    $containers = $(docker.exe ps -aq)
    if ($containers) {
        Log-Info "Cleaning up docker containers ..."
        $errMsg = $($containers | ForEach-Object { docker.exe rm -f $_ })
        if (-not $?) {
            Log-Warn "Could not remove docker containers: $errMsg"
        }
        # wait a while for rancher-wins to clean up processes
        Start-Sleep -Seconds 10
    }
}

function Remove-Kubernetes {
    Get-Process -ErrorAction Ignore -Name "rancher-wins-*" | ForEach-Object {
        Log-Info "Stopping process $($_.Name) ..."
        $_ | Stop-Process -ErrorAction Ignore -Force
    }
}

function Remove-FirewallRules {
    Get-NetFirewallRule -PolicyStore ActiveStore -Name "rancher-wins-*" -ErrorAction Ignore | ForEach-Object {
        Log-Info "Cleaning up firewall rule $($_.Name) ..."
        $_ | Remove-NetFirewallRule -ErrorAction Ignore | Out-Null
    }
}

function Remove-RancherWins {
	$service = Get-Service -Name "rancher-wins" -ErrorAction Ignore
	if ($service.Status -eq "Running") {
		Stop-Service $service.Name
	}

	Push-Location c:\etc\rancher
	$errMsg = $(.\wins.exe srv app run --unregister)
	if (-not $?) {
		Log-Warn "Could not unregister: $errMsg"
	}
	Pop-Location
}

function Remove-Links {
    try {
        # Removed the NAT as it isn't in the other one.
        Get-HnsNetwork | Where-Object { $_.Name -eq 'vxlan0' -or $_.Name -eq 'cbr0'} | Select-Object Name, ID | ForEach-Object {
            Log-Info "Cleaning up HnsNetwork $($_.Name) ..."
            hnsdiag delete networks ($_.ID)
        }
        Invoke-HNSRequest -Method "GET" -Type "policylists" | Where-Object { -not [string]::IsNullOrEmpty($_.Id) } | ForEach-Object {
            Log-Info "Cleaning up HNSPolicyList $($_.Id) ..."
            Invoke-HNSRequest -Method "DELETE" -Type "policylists" -Id $_.Id
        }
        ## This one doesn't exist in the previous version either. So we may try with NAT added and this removed.
        Get-HnsEndpoint  | Select-Object Name, ID | ForEach-Object {
            Log-Info "Cleaning up HnsEndpoint $($_.Name) ..."
            hnsdiag delete endpoints ($_.ID)
        }
    }
    catch {
        Log-Warn "Could not clean: $($_.Exception.Message)"
    }
}

function Remove-Paths {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory, ValueFromPipeline)]
        [string]
        $HostPathPrefix
    )
    PROCESS {
        $runPath = Join-Path $HostPathPrefix "run"
        $optPath = Join-Path $HostPathPrefix "opt"
        $varPath = Join-Path $HostPathPrefix "var"
        $etcPath = Join-Path $HostPathPrefix "etc"

		for ($num = 0; $num -lt 5; $num++){
			$sleep = $false

			Get-Item -ErrorAction Ignore -Path @(
				"$runPath\*"
				"$optPath\*"
				"$varPath\*"
				"$etcPath\*"
				"c:\ProgramData\docker\containers\*"
			) | ForEach-Object {
				Log-Info "Cleaning up data $($_.FullName) ..."
				try {
					$_ | Remove-Item -ErrorAction Ignore -Recurse -Force
				}
				catch {
					Log-Warn "Could not clean: $($_.Exception.Message)"
					$sleep = $true
				}
			}

			try {
				Log-Info "Cleaning up $runPath"
				Remove-Item -Path $runPath -ErrorAction Ignore -Recurse -Force
			} catch {
				$sleep = $true
			}

			try {
				Log-Info "Cleaning up $optPath"
				Remove-Item -Path $optPath -ErrorAction Ignore -Recurse -Force
			} catch {
				$sleep = $true
			}

			try {
				Log-Info "Cleaning up $varPath"
				Remove-Item -Path $varPath -ErrorAction Ignore -Recurse -Force
			} catch {
				$sleep = $true
			}

			try {
				Log-Info "Cleaning up $etcPath"
				Remove-Item -Path $etcPath -ErrorAction Ignore -Recurse -Force
			} catch {
				$sleep = $true
			}

			if ($sleep -eq $false) {
				break # No Remove-Item's were caught, we can move on
			}

			Sleep 5 # Sleep and try again
		}

        try {
            Log-Info "Restarting the Docker service"
            Stop-Service docker
            Start-Sleep -Seconds 5
            Start-Service docker
        }
        catch {
            Log-Fatal "Could not restart docker: $($_.Exception.Message)"
        }
    }
}

function Get-PrefixPath {
	$passedPrefixPath = "` + getPrefixPath() + `"
	if ([string]::IsNullOrEmpty($passedPrefixPath) -eq $false) {
		return $passedPrefixPath # cluster has a path prefix
	}
    Log-Info "Getting Windows prefix path"
    $rkeDefaultPrefix = "c:\"
    $dockerStatus = (docker info) | Out-Null
    try {
        $hostPrefixPath = (docker exec kubelet pwsh -c 'Get-ChildItem env:' 2>&1 | findstr RKE_NODE_PREFIX_PATH).Trim("RKE_NODE_PREFIX_PATH").Trim(" ")

        if ($dockerstatus.ExitCode -ne 0 -and !$hostPrefixPath) {
            $hostPrefixPath = $rkeDefaultPrefix
        }
        elseif ($hostPrefixPath) {
            if ($rkeDefaultPrefix -ine $hostPrefixPath) {
                $hostPrefixPath = $hostPrefixPath -Replace "/", "\"
                if ($hostPrefixPath.Chars($hostPrefixPath.Length - 1) -ne '\') {
                    $hostPrefixPath = $( $hostPrefixPath + '\' )
                }
            }
        }
        return $hostPrefixPath
    }
    catch {
        Log-Warn "Unable to find the host prefix path, it has been set to the default: 'c:\'"
    }

	return $rkeDefaultPrefix
}

function Spawn {
	Log-Info "Spawning script $PSCommandPath with default tasks to clean node"
	Start-Process -FilePath "powershell" -NoNewWindow -ArgumentList @("-ExecutionPolicy", "Bypass", "-File", $PSCommandPath)
}

function Logs {
	Get-EventLog -AppName Application -Source ` + NodeCleanupContainerName + ` -Newest 1000
}

# cleanup

if ([System.Diagnostics.EventLog]::SourceExists("` + NodeCleanupContainerName + `") -eq $false) {
	New-EventLog -LogName Application -Source ` + NodeCleanupContainerName + `
}
Log-Info "Start cleaning ..."

foreach ($task in $Tasks) {
    switch ($task) {
		"Docker" {
            # clean up docker container: docker rm -fv $(docker ps -qa)
            Remove-DockerContainers
        }
        "Kubernetes" {
            # clean up kubernetes components processes
            Remove-Kubernetes
        }
        "Firewall" {
            # clean up firewall rules
            Remove-FirewallRules
        }
        "Network" {
            # clean up links
            Remove-Links
        }
        "Rancher" {
            # clean up rancher-wins service
            Remove-RancherWins
        }
        "Paths" {
            # clean up data
            Get-PrefixPath | Remove-Paths
        }
        "Spawn" {
            # spawn this script with default tasks to clean everything
            Spawn
        }
		"Logs" {
            # spawn this script with default tasks to clean everything
            Logs
        }
    }
}

Log-Info "Finished!"
if ([System.Diagnostics.EventLog]::SourceExists("` + NodeCleanupContainerName + `") -eq $true) {
	Remove-EventLog -Source ` + NodeCleanupContainerName + `
}

`
}
