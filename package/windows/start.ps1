#Requires -Version 5.0

param (
    [parameter(Mandatory = $true)] [string]$server,
    [parameter(Mandatory = $true)] [string]$token,
    [parameter(Mandatory = $false)] [string]$caChecksum,
    [parameter(Mandatory = $false)] [string]$nodeName,
    [parameter(Mandatory = $false)] [string]$address,
    [parameter(Mandatory = $false)] [string]$internalAddress,
    [parameter(Mandatory = $false)] [string[]]$label,
    [parameter(Mandatory = $false)] [string]$customizeKubeletOptions,
    [parameter(Mandatory = $false)] [string]$customizeKubeProxyOptions,
    [parameter(Mandatory = $false)] [switch]$fgRun
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

Import-Module -Force -Name "$env:ProgramFiles\rancher\tool.psm1"

if (-not (Test-Path "C:\host")) {
    Log-Fatal "Please mount host `"C:\`" path to container `"C:\host`" path"
}

$RancherDir = "C:\host\etc\rancher"
$KubeDir = "C:\host\etc\kubernetes"
$CNIDir = "C:\host\etc\cni"
$NginxConfigDir = "C:\host\etc\nginx"
$KubeletRootDir = "C:\host\var\lib\kubelet"

$null = New-Item -Type Directory -Path $RancherDir -ErrorAction Ignore
$null = New-Item -Type Directory -Path $KubeDir -ErrorAction Ignore
$null = New-Item -Type Directory -Path $CNIDir -ErrorAction Ignore
$null = New-Item -Type Directory -Path $NginxConfigDir -ErrorAction Ignore
$null = New-Item -Type Directory -Path $KubeletRootDir -ErrorAction Ignore

# copy nginx #
try {
    Copy-Item -Force -Recurse -Path "$env:ProgramFiles\nginx\*.*" -Destination $NginxConfigDir
} catch {
    Log-Warn "Please empty host `"C:\etc\nginx`" path manually, because $($_.Exception.Message)"
}

# copy kubelet volume plugins #
try {
    $null = New-Item -Type Directory -Path "$KubeletRootDir\volumeplugins" -ErrorAction Ignore
    Copy-Item -Force -Recurse -Path "$env:ProgramFiles\kubelet\volumeplugins\*" -Destination "$KubeletRootDir\volumeplugins"
} catch {
    Log-Warn "Please empty host `"C:\var\lib\kubelet\volumeplugins`" path manually, because $($_.Exception.Message)"
}

# copy rancher agent artifacts #
try {
    Copy-Item -Force -Recurse -Path "$env:ProgramFiles\rancher\*.*" -Destination $RancherDir -Exclude @("run.ps1", "start.ps1")
} catch {
    Log-Fatal "Please empty host `"C:\etc\rancher`" path manually, because $($_.Exception.Message)"
}

# build rancher agent run.ps1 #
try {
    $runPSContent = Get-Content "$env:ProgramFiles\rancher\run.ps1"
    $runPSContent = $runPSContent -replace "<CATTLE_SERVER>",$server
    $runPSContent = $runPSContent -replace "<CATTLE_TOKEN>",$token
    if ($caChecksum) {
        $runPSContent = $runPSContent -replace "<CATTLE_CA_CHECKSUM>",$caChecksum
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_CA_CHECKSUM>",""
    }
    if ($PSBoundParameters['debug'] -or $PSBoundParameters['Debug']) {
        $runPSContent = $runPSContent -replace "<CATTLE_DEBUG>","true"
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_DEBUG>","false"
    }
    if ($nodeName) {
        $runPSContent = $runPSContent -replace "<CATTLE_NODE_NAME>",$nodeName
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_NODE_NAME>",""
    }
    if ($address) {
        $runPSContent = $runPSContent -replace "<CATTLE_ADDRESS>",$address
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_ADDRESS>",""
    }
    if ($internalAddress) {
        $runPSContent = $runPSContent -replace "<CATTLE_INTERNAL_ADDRESS>",$internalAddress
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_INTERNAL_ADDRESS>",""
    }
    if ($label) {
        $runPSContent = $runPSContent -replace "<CATTLE_NODE_LABEL>",($label -join ',')
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_NODE_LABEL>",""
    }
    if ($customizeKubeletOptions) {
        $runPSContent = $runPSContent -replace "<CATTLE_CUSTOMIZE_KUBELET_OPTIONS>",$customizeKubeletOptions
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_CUSTOMIZE_KUBELET_OPTIONS>",""
    }
    if ($customizeKubeProxyOptions) {
        $runPSContent = $runPSContent -replace "<CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS>",$customizeKubeProxyOptions
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS>",""
    }
    if ($fgRun) {
        $runPSContent = $runPSContent -replace "<CATTLE_AGENT_FG_RUN>","true"
    } else {
        $runPSContent = $runPSContent -replace "<CATTLE_AGENT_FG_RUN>","false"
    }

    $runPSContent | Out-File -Encoding ascii -Force -FilePath "$RancherDir\run.ps1"
} catch {
    Log-Fatal "Failed to build `"C:\etc\rancher\run.ps1`", because $($_.Exception.Message)"
}
