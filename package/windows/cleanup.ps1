#Requires -Version 5.0

param (
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

#########################################################################
## START main definition

$RancherDir = "C:\etc\rancher"
$KubeDir = "C:\etc\kubernetes"
$CNIDir = "C:\etc\cni"
$NginxConfigDir = "C:\etc\nginx"

Import-Module -Force -Name @("$RancherDir\hns.psm1", "$RancherDir\tool.psm1")

## END main definition
#########################################################################
## START main execution

# check identity #
if (-not (Is-Administrator)) {
    Log-Fatal "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option"
}

# remove rancher-agent service #
Get-Service -Name "rancher-agent" -ErrorAction Ignore | % {
    if ($_.Status -eq "Running") {
        Log-Info "Stopping rancher-agent service ..."
        $_ | Stop-Service -Force -PassThru -ErrorAction Ignore | Out-Null
    }

    $ret = Execute-Binary -FilePath "$RancherDir\agent.exe" -ArgumentList @("--unregister-service") -PassThru
    if (-not $ret.Success) {
        Log-Warn "Can't unregister rancher-agent service, $($ret.StdErr)"
    }
}

# stop kubernetes components processes #
Get-Process -ErrorAction Ignore -Name @(
    "flanneld*"
    "kubelet*"
    "kube-proxy*"
    "nginx*"
) | % {
    Log-Info "Stopping $($_.Name) process ..."
    $_ | Stop-Process -Force -ErrorAction Ignore
}

# clean up docker conatiner: docker rm -fv $(docker ps -qa) #
Log-Info "Removing Docker containers ..."
Execute-Binary -FilePath "docker.exe" -ArgumentList @('ps', '-qa') -PassThru | `
    Select-Object -ExpandProperty "StdOut" | `
    % { Execute-Binary -FilePath "docker.exe" -ArgumentList @('rm', '-fv', $_) }

# clean network interface #
Log-Info "Removing network interface ..."
Get-Env -Key "KUBE_NETWORK" | % {
    Clean-HNSNetworks -Names @{
        $_ = $True
    } | Out-Null
}
try {
    Get-HnsPolicyList | Remove-HnsPolicyList
} catch {}

# restore repair #
if (Test-Path "$RancherDir\GetGcePdName.dll") {
    Log-Info "Removing repaired 'Get-GcePdName' command ..."
    Remove-Module GetGcePdName -Force -ErrorAction Ignore | Out-Null
}

# clean firewall rules #
Log-Info "Removing firewall rules ..."
Remove-NetFirewallRule -ErrorAction Ignore -Name @(
    'OverlayTraffic4789UDP'
    'Kubelet10250TCP'
    'KubeProxy10256TCP'
) | Out-Null

# backup #
Log-Info "Backing up ..."
$date = (Get-Date).ToString('yyyyMMddHHmm')
Copy-Item -Recurse -Path $RancherDir -Destination "$RancherDir-bak-$date" -Exclude "connected" -Force -ErrorAction Ignore | Out-Null
Copy-Item -Recurse -Path $NginxConfigDir -Destination "$NginxConfigDir-bak-$date" -Force -ErrorAction Ignore | Out-Null
Copy-Item -Recurse -Path $CNIDir -Destination "$CNIDir-bak-$date" -Force -ErrorAction Ignore | Out-Null
Copy-Item -Recurse -Path $KubeDir -Destination "$KubeDir-bak-$date" -Force -ErrorAction Ignore | Out-Null

# clean up #
Log-Info "Cleaning up ..."
Remove-Item -Recurse -Force -ErrorAction Ignore -Path @(
    $KubeDir
    $CNIDir
    $NginxConfigDir
    "$RancherDir\*"
    "C:\etc\kube-flannel"
    "C:\run\flannel"
    "C:\var\log"
    "C:\var\lib\rancher"
    "C:\var\lib\kubelet"
    "C:\var\lib\cni"
) | Out-Null

# restart docker service #
Log-Info "Restarting Docker service ..."
Restart-Service -Name "docker" -ErrorAction Ignore | Out-Null

Log-Info "Finished!!!"

## END main execution
#########################################################################