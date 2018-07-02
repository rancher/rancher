#Requires -Version 5.0

param (
    [parameter(Mandatory = $true)] [string]$server,
    [parameter(Mandatory = $true)] [string]$token,
    [parameter(Mandatory = $true)] [string]$caChecksum,
    [parameter(Mandatory = $false)] [string]$nodeName,
    [parameter(Mandatory = $false)] [string]$address,
    [parameter(Mandatory = $false)] [string]$internalAddress,
    [parameter(Mandatory = $false)] [string]$label,
    [parameter(Mandatory = $false)] [string]$customizeKubeletOptions,
    [parameter(Mandatory = $false)] [string]$customizeKubeProxyOptions,
    [parameter(Mandatory = $false)] [switch]$fgRun
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

trap {
    [System.Console]::Error.Write("ERRO[0000] ")
    [System.Console]::Error.WriteLine($_)

    exit 1
}

if (-not (Test-Path "C:\host")) {
    throw "Please mount host `"C:\`" path to container `"C:\host`" path"
}

$rancherDir = "C:\host\etc\rancher"
$null = New-Item -Type Directory -Path $rancherDir -ErrorAction Ignore
$null = New-Item -Type Directory -Path "C:\host\etc\kubernetes" -ErrorAction Ignore
$null = New-Item -Type Directory -Path "C:\host\etc\cni" -ErrorAction Ignore

try {
    Copy-Item -Force -Path "$env:ProgramFiles\rancher\*.*" -Destination $rancherDir
    try {
        Remove-Item -Force -Path "$rancherDir\start.ps1" -ErrorAction Ignore
    } catch {}

    # build run.ps1 #
    $runPSContent = Get-Content "$env:ProgramFiles\rancher\run.ps1"
    $runPSContent = $runPSContent -replace "<CATTLE_SERVER>",$server
    $runPSContent = $runPSContent -replace "<CATTLE_TOKEN>",$token
    $runPSContent = $runPSContent -replace "<CATTLE_CA_CHECKSUM>",$caChecksum
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
        $runPSContent = $runPSContent -replace "<CATTLE_NODE_LABEL>",$label
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

    $runPSContent | Out-File -Encoding ascii -Force -FilePath "$rancherDir\run.ps1"
} catch {
    throw ("Please empty host `"C:\etc\rancher`" path manually, because {0}" -f $_.Exception.Message)
}

if (Test-Path "$rancherDir\connected") {
    [System.Console]::Out.WriteLine("WARN[0000] This host was or is connecting to a rancher server, please kept informed")

    Start-Sleep -s 5
}