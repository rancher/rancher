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

Import-Module "$RancherDir\hns.psm1" -Force

function get-env-var {
    param(
        [parameter(Mandatory = $true)] [string]$Key
    )

    $val = [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Process)
    if ($val) {
        return $val
    }

    return [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Machine)
}

## END main definition
#########################################################################
## START main execution

trap {
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO"
    Write-Host -NoNewline -ForegroundColor Gray "[0000] "
    Write-Host -ForegroundColor Gray $_

    popd

    exit 1
}


# check powershell #
if ($PSVersionTable.PSVersion.Major -lt 5) {
    throw "PowerShell version 5 or higher is required"
}

# check identity #
$currentPrincipal = new-object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option"
}

try {
    $svcAgent = Get-Service -Name "rancher-agent" -ErrorAction Ignore
    if ($svcAgent) {
        if ($svcAgent.Status -eq "Running") {
            $svcAgent | Stop-Service -Force -ErrorAction Ignore
        }

        pushd $RancherDir
        .\agent.exe --unregister-service *>$null
        popd
    }
} catch { }

try {
    # flanneld #
    $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process -Force | Out-Null
    }

    # kubelet #
    $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process -Force | Out-Null
    }

    # kube-proxy #
    $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process -Force | Out-Null
    }

    # controlplanes proxy #
    $process = Get-Process -Name "nginx*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process -Force | Out-Null
    }

    # docker stop #
    docker ps -q | % { docker stop $_ *>$null } *>$null

    # clean up rancher parts #
    docker rm kubernetes-binaries *>$null
    docker rm cni-binaries *>$null
} catch { }

try {
    $networkName = get-env-var "KUBE_NETWORK"
    if ($networkName) {
        $null = Clean-HNSNetworks -Names @{
            $networkName = $True
        }
    }

    # rancher #
    Remove-Item -Path "$CNIDir\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "$KubeDir\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "$RancherDir\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "$NginxConfigDir\*" -Recurse -Force -ErrorAction Ignore

    # component produces #
    # cni
    Remove-Item -Path "C:\etc\kube-flannel\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "C:\run\flannel\*" -Recurse -Force -ErrorAction Ignore
    Remove-NetFirewallRule -Name 'OverlayTraffic4789UDP' -ErrorAction Ignore

    # logs
    Remove-Item -Path "C:\var\log\*" -Recurse -Force -ErrorAction Ignore

    # kuberentes
    Remove-Item -Path "C:\var\lib\rancher\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "C:\var\lib\kubelet\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "C:\var\lib\cni\*" -Recurse -Force -ErrorAction Ignore
    Remove-Item -Path "C:\var\lib\etcd\*" -Recurse -Force -ErrorAction Ignore
    Remove-NetFirewallRule -Name @('Kubelet10250TCP', 'KubeProxy10256TCP') -ErrorAction Ignore
} catch { }

## END main execution
#########################################################################