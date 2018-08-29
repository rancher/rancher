#Requires -Version 5.0

param (
    [parameter(Mandatory = $false)] [string]$CATTLE_DEBUG = "<CATTLE_DEBUG>",
    [parameter(Mandatory = $false)] [string]$CATTLE_SERVER = "<CATTLE_SERVER>",
    [parameter(Mandatory = $false)] [string]$CATTLE_TOKEN = "<CATTLE_TOKEN>",
    [parameter(Mandatory = $false)] [string]$CATTLE_CA_CHECKSUM = "<CATTLE_CA_CHECKSUM>",
    [parameter(Mandatory = $false)] [string]$CATTLE_NODE_NAME = "<CATTLE_NODE_NAME>",
    [parameter(Mandatory = $false)] [string]$CATTLE_ADDRESS = "<CATTLE_ADDRESS>",
    [parameter(Mandatory = $false)] [string]$CATTLE_INTERNAL_ADDRESS = "<CATTLE_INTERNAL_ADDRESS>",
    [parameter(Mandatory = $false)] [string]$CATTLE_NODE_LABEL = "<CATTLE_NODE_LABEL>",
    [parameter(Mandatory = $false)] [string]$CATTLE_CUSTOMIZE_KUBELET_OPTIONS = "<CATTLE_CUSTOMIZE_KUBELET_OPTIONS>",
    [parameter(Mandatory = $false)] [string]$CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS = "<CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS>",
    [parameter(Mandatory = $false)] [string]$CATTLE_AGENT_FG_RUN = "<CATTLE_AGENT_FG_RUN>"
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

#########################################################################
## START main definitaion

$rancherDir = "C:\etc\rancher"

function scrape-text {
    param(
        [parameter(Mandatory = $false)] $Headers = @{"Cache-Control"="no-cache"},
        [parameter(Mandatory = $true)] [string]$Uri
    )

    try {
        $scraped = Invoke-WebRequest -Headers $Headers -UseBasicParsing -Uri $Uri
        return $scraped.Content
    } catch {
        log-error $_.Exception.Message
        return $null
    }
}

function scrape-json {
    param(
        [parameter(Mandatory = $true)] [string]$Uri
    )

    try {
        $scraped = Invoke-WebRequest -Headers @{"Accept"="application/json";"Cache-Control"="no-cache"} -UseBasicParsing -Uri $Uri
        return ($scraped.Content | ConvertFrom-Json)
    } catch {
        log-error $_.Exception.Message
        return $null
    }

}

function get-address {
    param(
        [parameter(Mandatory = $false)] [string]$Addr
    )

    if (-not $Addr) {
        try {
            $route = Find-NetRoute -RemoteIPAddress 8.8.8.8 | Select-Object -First 1
            return $route.IPAddress
        } catch {
            return ""
        }
    }

    switch ($Addr) {
        "awslocal" { return (scrape-text -Uri "http://169.254.169.254/latest/meta-data/local-ipv4") }
        "awspublic" { return (scrape-text -Uri "http://169.254.169.254/latest/meta-data/public-ipv4") }
        "doprivate" { return (scrape-text -Uri "http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address") }
        "dopublic" { return (scrape-text -Uri "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address") }
        "azprivate" { return (scrape-text -Headers @{"Metadata"="true"} -Uri "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-08-01&format=text") }
        "azpublic" { return (scrape-text -Headers @{"Metadata"="true"} -Uri "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text") }
        "gceinternal" { return (scrape-text -Headers @{"Metadata-Flavor"="Google"} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip") }
        "gceexternal" { return (scrape-text -Headers @{"Metadata-Flavor"="Google"} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip") }
        "packetlocal" { return (scrape-text -Uri "https://metadata.packet.net/2009-04-04/meta-data/local-ipv4") }
        "packetpublic" { return (scrape-text -Uri "https://metadata.packet.net/2009-04-04/meta-data/public-ipv4") }
        "ipify" { return (scrape-text -Uri "https://api.ipify.org") }
    }

    return $Addr
}

function set-env-var {
    param(
        [parameter(Mandatory = $true)] [string]$Key,
        [parameter(Mandatory = $false)] [string]$Value = ""
    )

    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Process)
    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Machine)
}

function get-agent-service {
    $svcRancherAgt = Get-Service -Name "rancher-agent" -ErrorAction Ignore
    return $svcRancherAgt
}

## END main definitaion
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
    throw "PowerShell version 5 or higher is required, exit"
}

# check identity #
$currentPrincipal = new-object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option, exit"
}

# set http client #
try {
    add-type @"
    using System.Net;
    using System.Security.Cryptography.X509Certificates;
    public class TrustAllCertsPolicy : ICertificatePolicy {
        public bool CheckValidationResult(
            ServicePoint srvPoint, X509Certificate certificate,
            WebRequest request, int certificateProblem) {
            return true;
        }
    }
"@
} catch {}
[System.Net.ServicePointManager]::CertificatePolicy = New-Object TrustAllCertsPolicy
[Net.ServicePointManager]::SecurityProtocol = @([Net.SecurityProtocolType]::SystemDefault, [Net.SecurityProtocolType]::Ssl3, [Net.SecurityProtocolType]::Tls, [Net.SecurityProtocolType]::Tls11, [Net.SecurityProtocolType]::Tls12)

# check docker running #
$dockerNPipe = Get-ChildItem //./pipe/ | ? Name -eq "docker_engine"
if (-not $dockerNPipe) {
    throw "Please run docker daemon with host `"npipe:////./pipe//docker_engine`", exit"
}

$svcAgent = get-agent-service
if ($svcAgent -and ($svcAgent.Status -eq "Running")) {
    throw "rancher agent is already on running"
}

# check cattle server address #
if (-not $CATTLE_SERVER) {
    throw "-Server is a required option, exit"
}

# check cattle server token #
if (-not $CATTLE_TOKEN) {
    throw "-Token is a required option, exit"
}

# check node name #
if (-not $CATTLE_NODE_NAME) {
    $CATTLE_NODE_NAME = hostname
}
if (-not $CATTLE_NODE_NAME) {
    throw "-NodeName is a required option, exit"
}
$CATTLE_NODE_NAME = $CATTLE_NODE_NAME.ToLower()

# check node address #
$CATTLE_INTERNAL_ADDRESS = get-address -Addr $CATTLE_INTERNAL_ADDRESS
if (-not $CATTLE_ADDRESS) {
    $CATTLE_ADDRESS = $CATTLE_INTERNAL_ADDRESS
}
if (-not $CATTLE_ADDRESS) {
    throw "-Address is a required option, exit"
}

# download cattle server CA #
$SSL_CERT_DIR = "C:\etc\kubernetes\ssl\certs"
$temp = New-TemporaryFile
$cacerts = $null
try {
    $cacerts = (scrape-json -Uri "$CATTLE_SERVER/v3/settings/cacerts").value
} catch {}

if (-not $cacerts) {
    throw "Can't get cattle server CA from $CATTLE_SERVER, exit"
}
$cacerts + "`n" | Out-File -NoNewline -Encoding ascii -FilePath $temp.FullName
$tempHasher = Get-FileHash -LiteralPath $temp.FullName -Algorithm SHA256
if ($tempHasher.Hash.ToLower() -ne $CATTLE_CA_CHECKSUM.ToLower()) {
    $temp.Delete()
    throw "Actual cattle server CA checksum is $($tempHasher.Hash.ToLower()), $CATTLE_SERVER/v3/settings/cacerts does not match $($CATTLE_CA_CHECKSUM.ToLower()), exit"
}
rm -Force -Recurse "$SSL_CERT_DIR\serverca" -ErrorAction Ignore | Out-Null
$null = New-Item -Force -Type Directory -Path $SSL_CERT_DIR -ErrorAction Ignore
$temp.MoveTo("$SSL_CERT_DIR\serverca")

# add labels #
$labels = @()
$windowsCurrentVersion = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' -ErrorAction Ignore)
if ($windowsCurrentVersion) {
    $labels += @("rke.cattle.io/windows-release-id=$($windowsCurrentVersion.ReleaseId)")
    $labels += @("rke.cattle.io/windows-major-version=$($windowsCurrentVersion.CurrentMajorVersionNumber)")
    $labels += @("rke.cattle.io/windows-minor-version=$($windowsCurrentVersion.CurrentMinorVersionNumber)")
    $labels += @("rke.cattle.io/windows-kernel-version=$($windowsCurrentVersion.BuildLabEx)")
    $labels += @("rke.cattle.io/windows-build=$($windowsCurrentVersion.CurrentBuild)")
}
if ($CATTLE_NODE_LABEL) {
    $labels += @(($CATTLE_NODE_LABEL -split ","))
}
if ($labels.Count -gt 0) {
    $usingLabel = @()
    $labels | % {
        $item = $_
        if ($item) {
            $usingLabel += @($item)
        }
    }
    $CATTLE_NODE_LABEL = $usingLabel -join ","
}

# set environment variables #
#$env:DOCKER_HOST = "npipe:////./pipe//docker_engine"
#$env:SSL_CERT_DIR = $SSL_CERT_DIR
#$env:CATTLE_INTERNAL_ADDRESS = $CATTLE_INTERNAL_ADDRESS
#$env:CATTLE_NODE_NAME = $CATTLE_NODE_NAME
#$env:CATTLE_ADDRESS = $CATTLE_ADDRESS
#$env:CATTLE_ROLE = "worker"
#$env:CATTLE_SERVER = $CATTLE_SERVER
#$env:CATTLE_TOKEN = $CATTLE_TOKEN
#$env:CATTLE_DEBUG = $CATTLE_DEBUG
#$env:CATTLE_NODE_LABEL = $CATTLE_NODE_LABEL
set-env-var -Key "DOCKER_HOST" -Value "npipe:////./pipe/docker_engine"
set-env-var -Key "SSL_CERT_DIR" -Value $SSL_CERT_DIR
set-env-var -Key "CATTLE_INTERNAL_ADDRESS" -Value $CATTLE_INTERNAL_ADDRESS
set-env-var -Key "CATTLE_NODE_NAME" -Value $CATTLE_NODE_NAME
set-env-var -Key "CATTLE_ADDRESS" -Value $CATTLE_ADDRESS
set-env-var -Key "CATTLE_ROLE" -Value "worker"
set-env-var -Key "CATTLE_SERVER" -Value $CATTLE_SERVER
set-env-var -Key "CATTLE_TOKEN" -Value $CATTLE_TOKEN
set-env-var -Key "CATTLE_DEBUG" -Value $CATTLE_DEBUG
set-env-var -Key "CATTLE_NODE_LABEL" -Value $CATTLE_NODE_LABEL
set-env-var -Key "CATTLE_CUSTOMIZE_KUBELET_OPTIONS" -Value $CATTLE_CUSTOMIZE_KUBELET_OPTIONS
set-env-var -Key "CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS" -Value $CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS

# run rancher-agent #
pushd $rancherDir
.\agent.exe --unregister-service *>$null
if ($CATTLE_AGENT_FG_RUN -eq "true") {
    .\agent.exe
} else {
    .\agent.exe --register-service *>$null
    $svcAgent = get-agent-service
    if (-not $svcAgent) {
        throw "Can't start rancher agent service, because no exist"
    } else {
        Start-Service -Name "rancher-agent" -ErrorAction Ignore
        if (-not $?) {
            throw "Start rancher agent failed"
        }
    }

    .\log.ps1
}

## END main execution
#########################################################################