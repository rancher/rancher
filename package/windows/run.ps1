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

$RancherDir = "C:\etc\rancher"
$KubeDir = "C:\etc\kubernetes"
$CNIDir = "C:\etc\cni"

$null = New-Item -Force -Type Directory -Path $RancherDir -ErrorAction Ignore
$null = New-Item -Force -Type Directory -Path $KubeDir -ErrorAction Ignore
$null = New-Item -Force -Type Directory -Path $CNIDir -ErrorAction Ignore

Import-Module -Force -Name "$RancherDir\tool.psm1"

function get-address {
    param(
        [parameter(Mandatory = $false)] [string]$Addr
    )

    if (-not $Addr) {
        return ""
    }

    # If given address is a network interface on the system, retrieve configured IP on that interface (only the first configured IP is taken)
    try {
        $na = Get-NetAdapter | ? Name -eq $Addr
        if ($na) {
            return (Get-NetIPAddress -InterfaceIndex $na.ifIndex -AddressFamily IPv4).IPAddress
        }
    } catch {}

    # Loop through cloud provider options to get IP from metadata, if not found return given value
    switch ($Addr) {
        "awslocal" { return (Scrape-Content -Uri "http://169.254.169.254/latest/meta-data/local-ipv4") }
        "awspublic" { return (Scrape-Content -Uri "http://169.254.169.254/latest/meta-data/public-ipv4") }
        "doprivate" { return (Scrape-Content -Uri "http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address") }
        "dopublic" { return (Scrape-Content -Uri "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address") }
        "azprivate" { return (Scrape-Content -Headers @{"Metadata"="true"} -Uri "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-08-01&format=text") }
        "azpublic" { return (Scrape-Content -Headers @{"Metadata"="true"} -Uri "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text") }
        "gceinternal" { return (Scrape-Content -Headers @{"Metadata-Flavor"="Google"} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip?alt=json") }
        "gceexternal" { return (Scrape-Content -Headers @{"Metadata-Flavor"="Google"} -Uri "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip?alt=json") }
        "packetlocal" { return (Scrape-Content -Uri "https://metadata.packet.net/2009-04-04/meta-data/local-ipv4") }
        "packetpublic" { return (Scrape-Content -Uri "https://metadata.packet.net/2009-04-04/meta-data/public-ipv4") }
        "ipify" { return (Scrape-Content -Uri "https://api.ipify.org") }
    }

    return $Addr
}

## END main definitaion
#########################################################################
## START main execution

# check identity #
if (-not (Is-Administrator)) {
    Log-Fatal "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option"
}

# check rancher-agent running #
$svcAgent = Get-Service -Name "rancher-agent" -ErrorAction Ignore
if ($svcAgent -and ($svcAgent.Status -eq "Running")) {
    Log-Fatal "rancher agent is already on running"
}

# check msiscsi servcie running #
$svcMsiscsi = Get-Service -Name "msiscsi" -ErrorAction Ignore
if ($svcMsiscsi -and ($svcMsiscsi.Status -ne "Running")) {
    Set-Service -Name "msiscsi" -StartupType Automatic
    Start-Service -Name "msiscsi" -ErrorAction Ignore
    if (-not $?) {
        Log-Warn "Failed to start msiscsi service, you may not be able to use the iSCSI flexvolume properly"
    }
}

# check docker running #
$dockerNPipe = Get-ChildItem //./pipe/ -ErrorAction Ignore | ? Name -eq "docker_engine"
if (-not $dockerNPipe) {
    Log-Warn "Default docker named pipe is not found"
    Log-Warn "Please bind mount in the docker socket to //./pipe/docker_engine if docker errors occur"
    Log-Warn "example:  docker run -v //./pipe/docker_engine://./pipe/docker_engine ..."
}

# check docker release #
$ret = Execute-Binary -FilePath "docker.exe" -ArgumentList @("version", "-f", "{{.Server.Platform.Name}}") -PassThru
if (-not ($ret.StdOut -like '*Enterprise*')) {
    Log-Fatal "Only support with Docker EE"
}

# check system locale #
$sysLocale = Get-WinSystemLocale | Select-Object -ExpandProperty "IetfLanguageTag"
if (-not $sysLocale.StartsWith('en-')) {
    Log-Fatal "Only support with English System Locale"
}

# check network count #
$vNetAdapters = Get-HnsNetwork | Select-Object -ExpandProperty "Subnets" | Select-Object -ExpandProperty "GatewayAddress"
$allNetAdapters = Get-WmiObject -Class Win32_NetworkAdapterConfiguration -Filter "IPEnabled=True" | Sort-Object Index | % { $_.IPAddress[0] } | ? { -not ($vNetAdapters -contains $_) }
if (($allNetAdapters | Measure-Object | Select-Object -ExpandProperty "Count") -gt 1) {
    if (-not $CATTLE_INTERNAL_ADDRESS) {
        Log-Warn "More than 1 network interfaces are found: $($allNetAdapters -join ", ")"
        Log-Warn "Please indicate -internalAddress when adding"
    }
}

# check cattle server address #
if (-not $CATTLE_SERVER) {
    Log-Fatal "-server is a required option"
} else {
    try {
        $null = Scrape-Content -Uri "$CATTLE_SERVER/ping" -SkipCertificateCheck
    } catch {
        Log-Fatal "$CATTLE_SERVER is not accessible $($_.Exception.Message)"
    }
}

# check cattle server token #
if (-not $CATTLE_TOKEN) {
    Log-Fatal "-token is a required option"
}

# check node name #
if (-not $CATTLE_NODE_NAME) {
    $CATTLE_NODE_NAME = hostname
}
if (-not $CATTLE_NODE_NAME) {
    Log-Fatal "-nodeName is a required option"
}
$CATTLE_NODE_NAME = $CATTLE_NODE_NAME.ToLower()

# check node address #
$CATTLE_ADDRESS = get-address -Addr $CATTLE_ADDRESS
$CATTLE_INTERNAL_ADDRESS = get-address -Addr $CATTLE_INTERNAL_ADDRESS
if (-not $CATTLE_INTERNAL_ADDRESS) {
    try {
        $route = Find-NetRoute -RemoteIPAddress 8.8.8.8 | Select-Object -First 1
        $CATTLE_INTERNAL_ADDRESS = $route.IPAddress
    } catch {
        Log-Warn "Can't detect internal IP automatically"
    }
}
if (-not $CATTLE_INTERNAL_ADDRESS) {
    Log-Fatal "-internalAddress is a required option"
}
if (-not $CATTLE_ADDRESS) {
    $CATTLE_ADDRESS = $CATTLE_INTERNAL_ADDRESS
}

# download cattle server CA #
$SSL_CERT_DIR = "C:\etc\kubernetes\ssl\certs"
if ($CATTLE_CA_CHECKSUM) {
    $temp = New-TemporaryFile
    $cacerts = (Scrape-Content -Headers @{"Cache-Control"="no-cache";"Accept"="application/json"} -Uri "$CATTLE_SERVER/v3/settings/cacerts" -SkipCertificateCheck).value

    if (-not $cacerts) {
        Log-Fatal "Can't get cattle server CA from $CATTLE_SERVER"
    }
    $cacerts + "`n" | Out-File -NoNewline -Encoding ascii -FilePath $temp.FullName
    $tempHasher = Get-FileHash -LiteralPath $temp.FullName -Algorithm SHA256
    if ($tempHasher.Hash.ToLower() -ne $CATTLE_CA_CHECKSUM.ToLower()) {
        $temp.Delete()
        Log-Fatal "Actual cattle server CA checksum is $($tempHasher.Hash.ToLower()), $CATTLE_SERVER/v3/settings/cacerts does not match $($CATTLE_CA_CHECKSUM.ToLower())"
    }
    rm -Force -Recurse "$SSL_CERT_DIR\serverca" -ErrorAction Ignore
    $null = New-Item -Force -Type Directory -Path $SSL_CERT_DIR -ErrorAction Ignore
    $temp.MoveTo("$SSL_CERT_DIR\serverca")

    #import the self-signed certificate#
    $caBytes = $null
    Get-Content "$SSL_CERT_DIR\serverca" | % {
        if ($_ -match '-+BEGIN CERTIFICATE-+') {
            $caBytes = @()
        } elseif ($_ -match '-+END CERTIFICATE-+') {
            $caTemp = New-TemporaryFile
            Set-Content -Value $caBytes -Path $caTemp.FullName -Encoding Byte
            Import-Certificate -CertStoreLocation 'Cert:\LocalMachine\Root' -FilePath $caTemp.FullName | Out-Null
            $caTemp.Delete()
        } else {
            $caBytes += [Convert]::FromBase64String($_)
        }
    }
}

# add labels #
$labels = @()
$windowsCurrentVersion = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\' -ErrorAction Ignore)
if ($windowsCurrentVersion) {
    $versionTag = "$($windowsCurrentVersion.CurrentMajorVersionNumber).$($windowsCurrentVersion.CurrentMinorVersionNumber).$($windowsCurrentVersion.CurrentBuildNumber).$($windowsCurrentVersion.UBR)"
    $labels += @("rke.cattle.io/windows-version=$versionTag")
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
Set-Env -Key "DOCKER_HOST" -Value "npipe:////./pipe/docker_engine"
Set-Env -Key "SSL_CERT_DIR" -Value $SSL_CERT_DIR
Set-Env -Key "CATTLE_INTERNAL_ADDRESS" -Value $CATTLE_INTERNAL_ADDRESS
Set-Env -Key "CATTLE_NODE_NAME" -Value $CATTLE_NODE_NAME
Set-Env -Key "CATTLE_ADDRESS" -Value $CATTLE_ADDRESS
Set-Env -Key "CATTLE_ROLE" -Value "worker"
Set-Env -Key "CATTLE_SERVER" -Value $CATTLE_SERVER
Set-Env -Key "CATTLE_TOKEN" -Value $CATTLE_TOKEN
Set-Env -Key "CATTLE_DEBUG" -Value $CATTLE_DEBUG
Set-Env -Key "CATTLE_NODE_LABEL" -Value $CATTLE_NODE_LABEL
Set-Env -Key "CATTLE_CUSTOMIZE_KUBELET_OPTIONS" -Value $CATTLE_CUSTOMIZE_KUBELET_OPTIONS
Set-Env -Key "CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS" -Value $CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS
Set-Env -Key "CATTLE_AGENT_FG_RUN" -Value $CATTLE_AGENT_FG_RUN

# run rancher-agent #
$ret = Execute-Binary -FilePath "$RancherDir\agent.exe" -ArgumentList @("--unregister-service") -PassThru
if (-not $ret.Success) {
    Log-Warn "Can't unregister rancher-agent service, $($ret.StdErr)"
}
if ($CATTLE_AGENT_FG_RUN -ne "true") {
    $ret = Execute-Binary -FilePath "$RancherDir\agent.exe" -ArgumentList @("--register-service") -PassThru
    if (-not $ret.Success) {
        Log-Fatal "Can't register rancher-agent service, $($ret.StdErr)"
    }

    Start-Service -Name "rancher-agent" -ErrorAction Ignore
    if (-not $?) {
        Log-Fatal "Start rancher agent failed"
    }

    Invoke-Expression -Command "$RancherDir\log.ps1"

    exit 0
}
Execute-Binary -FilePath "$RancherDir\agent.exe"

## END main execution
#########################################################################