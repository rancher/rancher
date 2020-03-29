<#
	run.ps1 executes the agent
 #>

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

Import-Module -WarningAction Ignore -Name "$PSScriptRoot\utils.psm1"

function Get-Address
{
    param(
        [parameter(Mandatory = $false)] [string]$Addr
    )

    if (-not $Addr) {
        return ""
    }

    # If given address is a network interface on the system, retrieve configured IP on that interface (only the first configured IP is taken)
    try {
        $getAdapterJson = wins.exe cli net get --name $Addr
        if ($?) {
            $namedNetwork = $getAdapterJson | ConvertTo-JsonObj
            if ($namedNetwork) {
                return $namedNetwork.AddressCIDR -replace "/32",""
            }
        }
    } catch {}

    # Repair the container route for `169.254.169.254` before cloud provider query
    $actualGateway = $(route.exe print 0.0.0.0 | Where-Object {$_ -match '0\.0\.0\.0.*[a-z]'} | Select-Object -First 1 | ForEach-Object {($_ -replace '0\.0\.0\.0|[a-z]|\s+',' ').Trim() -split ' '} | Select-Object -First 1)
    $expectedGateway = $(route.exe print 169.254.169.254 | Where-Object {$_ -match '169\.254\.169\.254'} | Select-Object -First 1 | ForEach-Object {($_ -replace '169\.254\.169\.254|255\.255\.255\.255|[a-z]|\s+',' ').Trim() -split ' '} | Select-Object -First 1)
    if ($actualGateway -ne $expectedGateway) {
        $errMsg = $(route.exe add 169.254.169.254 MASK 255.255.255.255 $actualGateway METRIC 1)
        if (-not $?) {
            Log-Error "Could not repair contain route for using cloud provider"
        }
    }

    # Loop through cloud provider options to get IP from metadata, if not found return given value
    switch ($Addr)
    {
        "awslocal" {
            return $(curl.exe -s "http://169.254.169.254/latest/meta-data/local-ipv4")
        }
        "awspublic" {
            return $(curl.exe -s "http://169.254.169.254/latest/meta-data/public-ipv4")
        }
        "doprivate" {
            return $(curl.exe -s "http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address")
        }
        "dopublic" {
            return $(curl.exe -s "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address")
        }
        "azprivate" {
            return $(curl.exe -s -H "Metadata:true" "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-08-01&format=text")
        }
        "azpublic" {
            return $(curl.exe -s -H "Metadata:true" "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text")
        }
        "gceinternal" {
            return $(curl.exe -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip?alt=json")
        }
        "gceexternal" {
            return $(curl.exe -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip?alt=json")
        }
        "packetlocal" {
            return $(curl.exe -s "https://metadata.packet.net/2009-04-04/meta-data/local-ipv4")
        }
        "packetpublic" {
            return $(curl.exe -s "https://metadata.packet.net/2009-04-04/meta-data/public-ipv4")
        }
        "scwprivate" {
            return $(curl.exe -s http://169.254.42.42/conf | Where-Object {$_ -match '^PRIVATE_IP='} | Select-Object -First 1 | ForEach-Object {($_ -replace '^PRIVATE_IP=','').Trim()})
        }
        "scwpublic" {
            return $(curl.exe -s http://169.254.42.42/conf | Where-Object {$_ -match '^PUBLIC_IP_ADDRESS='} | Select-Object -First 1 | ForEach-Object {($_ -replace '^PUBLIC_IP_ADDRESS=','').Trim()})
        }
        "ipify" {
            return $(curl.exe -s "https://api.ipify.org")
        }
    }

    return $Addr
}

# required envs
Set-Env -Key "DOCKER_HOST" -Value "npipe:////./pipe/docker_engine"
Set-Env -Key "CATTLE_ROLE" -Value "worker"

# clean up
$CLUSTER_CLEANUP = Get-Env -Key "CLUSTER_CLEANUP"
if ($CLUSTER_CLEANUP -eq "true")
{
    Start-Process -NoNewWindow -Wait -FilePath "c:\etc\rancher\agent.exe"
    exit 0
}

# init parameters
$CATTLE_SERVER = Get-Env -Key "CATTLE_SERVER"
$CATTLE_TOKEN = Get-Env -Key "CATTLE_TOKEN"
$CATTLE_NODE_NAME = Get-Env -Key "CATTLE_NODE_NAME"
$CATTLE_ADDRESS = Get-Env -Key "CATTLE_ADDRESS"
$CATTLE_INTERNAL_ADDRESS = Get-Env -Key "CATTLE_INTERNAL_ADDRESS"
$CATTLE_CA_CHECKSUM = Get-Env -Key "CATTLE_CA_CHECKSUM"
$CATTLE_NODE_LABEL = @()
$CATTLE_NODE_TAINTS = @()

# parse arguments
$vals = $null
for ($i = $args.Length; $i -ge 0; $i--)
{
    $arg = $args[$i]
    switch -regex ($arg)
    {
        '^(-d|--debug)$' {
            Set-Env -Key "CATTLE_DEBUG" -Value "true"
            $vals = $null
        }
        '^(-s|--server)$' {
            $CATTLE_SERVER = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-t|--token)$' {
            $CATTLE_TOKEN = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-c|--ca-checksum)$' {
            $CATTLE_CA_CHECKSUM = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-all|--all-roles)$' {
            $vals = $null
        }
        '^(-e|--etcd)$' {
            $vals = $null
        }
        '^(-w|--worker)$' {
            $vals = $null
        }
        '^(-p|--controlplane)$' {
            $vals = $null
        }
        '^(-r|--node-name)$' {
            $CATTLE_NODE_NAME = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-n|--no-register)$' {
            Set-Env -Key "CATTLE_AGENT_CONNECT" -Value "true"
            $vals = $null
        }
        '^(-a|--address)$' {
            $CATTLE_ADDRESS = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-i|--internal-address)$' {
            $CATTLE_INTERNAL_ADDRESS = ($vals | Select-Object -First 1)
            $vals = $null
        }
        '^(-l|--label)$' {
            if ($vals) {
                $CATTLE_NODE_LABEL += $vals
            }
            $vals = $null
        }
        '^(-o|--only-write-certs)$' {
            Set-Env -Key "CATTLE_WRITE_CERT_ONLY" -Value "true"
            $vals = $null
        }
        '^--taints$' {
            if ($vals) {
                $CATTLE_NODE_TAINTS += $vals
            }
            $vals = $null
        }
        default {
            if ($vals) {
                $vals = ,$arg + $vals
            } else {
                $vals = @($arg)
            }
        }
    }
}

# use the register token to request wins server
if (Test-Path -PathType Leaf -Path "c:\cattle-credentials\token")
{
    $token = Get-Content -Path "c:\cattle-credentials\token" -Raw -ErrorAction Ignore
    Set-Env -Key "WINS_AUTH_TOKEN" -Value $token
}

# check docker npipe
$CATTLE_CLUSTER = Get-Env -Key "CATTLE_CLUSTER"
if ($CATTLE_CLUSTER -ne "true")
{
    $dockerNPipe = Get-ChildItem //./pipe/ -ErrorAction Ignore | ? Name -eq "docker_engine"
    if (-not $dockerNPipe) {
        Log-Warn "Default docker named pipe is not found"
        Log-Warn "Please bind mount in the docker named pipe to //./pipe/docker_engine if docker errors occur"
        Log-Warn "example: docker run -v //./pipe/custom_docker_named_pipe://./pipe/docker_engine ..."
    }
}

# get address
$CATTLE_ADDRESS = Get-Address -Addr $CATTLE_ADDRESS
$CATTLE_INTERNAL_ADDRESS = Get-Address -Addr $CATTLE_INTERNAL_ADDRESS

# get default network metadata when nodeName or address is blank
if ((-not $CATTLE_NODE_NAME) -or (-not $CATTLE_ADDRESS))
{
    $getAdapterJson = wins.exe cli net get
    if ($?) {
        $defaultNetwork = $getAdapterJson | ConvertTo-JsonObj
        if ($defaultNetwork) {
            if (-not $CATTLE_NODE_NAME) {
                $CATTLE_NODE_NAME = $defaultNetwork.HostName
                $CATTLE_NODE_NAME = $CATTLE_NODE_NAME.ToLower()
            }

            if (-not $CATTLE_ADDRESS) {
                $CATTLE_ADDRESS = $defaultNetwork.AddressCIDR -replace "/32",""
            }
        } else {
            Log-Warn "Could not convert '$getAdapterJson' to json object"
        }
    } else {
        Log-Warn "Could not get host network metadata: $getAdapterJson"
    }
}

# check token and address
$CATTLE_K8S_MANAGED = Get-Env -Key "CATTLE_K8S_MANAGED"
if ($CATTLE_K8S_MANAGED -ne "true")
{
    if (-not $CATTLE_TOKEN) {
        Log-Fatal "--token is a required option"
    }
    if (-not $CATTLE_ADDRESS) {
        Log-Fatal "--address is a required option"
    }
}

# check rancher server address
if (-not $CATTLE_SERVER)
{
    Log-Fatal "--server is a required option"
}

# check rancher server
try
{
    curl.exe --insecure -s -fL "$CATTLE_SERVER/ping" | Out-Null
    if ($?) {
        Log-Info "$CATTLE_SERVER is accessible"
    } else {
        Log-Fatal "$CATTLE_SERVER is not accessible"
    }
}
catch
{
    Log-Fatal "$CATTLE_SERVER is not accessible: $($_.Exception.Message)"
}

# download cattle server CA
if ($CATTLE_CA_CHECKSUM)
{
    $sslCertDir = Get-Env -Key "SSL_CERT_DIR"
    $server = $CATTLE_SERVER
    $caChecksum = $CATTLE_CA_CHECKSUM
    $temp = New-TemporaryFile
    $cacerts = $null
    try {
        $cacerts = $(curl.exe --insecure -s -fL "$server/v3/settings/cacerts" | ConvertTo-JsonObj).value
    } catch {}
    if (-not $cacerts) {
        Log-Fatal "Could not get cattle server CA from $server"
    }

    $cacerts + "`n" | Out-File -NoNewline -Encoding ascii -FilePath $temp.FullName
    $tempHasher = Get-FileHash -LiteralPath $temp.FullName -Algorithm SHA256
    if ($tempHasher.Hash.ToLower() -ne $caChecksum.ToLower()) {
        $temp.Delete()
        Log-Fatal "Actual cattle server CA checksum is $($tempHasher.Hash.ToLower()), $server/v3/settings/cacerts does not match $($caChecksum.ToLower())"
    }
    Remove-Item -Force -Recurse -Path "$sslCertDir\serverca" -ErrorAction Ignore
    New-Item -Force -ItemType Directory -Path $sslCertDir -ErrorAction Ignore | Out-Null
    $temp.MoveTo("$sslCertDir\serverca")

    # import the self-signed certificate
    $caBytes = $null
    Get-Content "$sslCertDir\serverca" | % {
        if ($_ -match '-+BEGIN CERTIFICATE-+') {
            $caBytes = @()
        } elseif ($_ -match '-+END CERTIFICATE-+') {
            $caTemp = New-TemporaryFile
            $caString = [Convert]::ToBase64String($caBytes)
            Set-Content -Value $caString -Path $caTemp.FullName
            certoc.exe -addstore root $caTemp.FullName | Out-Null
            if (-not $?) {
                $caTemp.Delete()
                Log-Fatal "Failed to import rancher server certificates to Root"
            }
            $caTemp.Delete()
        } else {
            $caBytes += [Convert]::FromBase64String($_)
        }
    }

    $CATTLE_SERVER_HOSTNAME = ([System.Uri]"$server").Host
    $CATTLE_SERVER_HOSTNAME_WITH_PORT = ([System.Uri]"$server").Authority

    # windows path could not allow colons
    $CATTLE_SERVER_HOSTNAME_WITH_PORT = $CATTLE_SERVER_HOSTNAME_WITH_PORT -replace ":", ""

    $dockerCertsPath = "c:\etc\docker\certs.d\$CATTLE_SERVER_HOSTNAME_WITH_PORT"
    New-Item -Force -ItemType Directory -Path $dockerCertsPath -ErrorAction Ignore | Out-Null
    Copy-Item -Force -Path "$sslCertDir\serverca" -Destination "$dockerCertsPath\ca.crt" -ErrorAction Ignore
}

# add labels
$getVersionJson = wins.exe cli host get-version
if ($?) {
    $windowsCurrentVersion = $getVersionJson | ConvertTo-JsonObj
    if ($windowsCurrentVersion) {
        $versionTag = "$($windowsCurrentVersion.CurrentMajorVersionNumber).$($windowsCurrentVersion.CurrentMinorVersionNumber).$($windowsCurrentVersion.CurrentBuildNumber).$($windowsCurrentVersion.UBR)"
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-version=$versionTag")
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-release-id=$($windowsCurrentVersion.ReleaseId)")
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-major-version=$($windowsCurrentVersion.CurrentMajorVersionNumber)")
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-minor-version=$($windowsCurrentVersion.CurrentMinorVersionNumber)")
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-kernel-version=$($windowsCurrentVersion.BuildLabEx)")
        $CATTLE_NODE_LABEL += @("rke.cattle.io/windows-build=$($windowsCurrentVersion.CurrentBuild)")
    } else {
        Log-Warn "Could not convert Windows Current Version JSON '$getVersionJson' to object"
    }
} else {
    Log-Warn "Could not get host version: $getVersionJson"
}

# set environment variables
Set-Env -Key "CATTLE_SERVER" -Value $CATTLE_SERVER
Set-Env -Key "CATTLE_TOKEN" -Value $CATTLE_TOKEN
Set-Env -Key "CATTLE_ADDRESS" -Val $CATTLE_ADDRESS
Set-Env -Key "CATTLE_INTERNAL_ADDRESS" -Val $CATTLE_INTERNAL_ADDRESS
Set-Env -Key "CATTLE_NODE_NAME" -Value $CATTLE_NODE_NAME
Set-Env -Key "CATTLE_NODE_LABEL" -Value $($CATTLE_NODE_LABEL -join ",")
Set-Env -Key "CATTLE_NODE_TAINTS" -Value $($CATTLE_NODE_TAINTS -join ",")

# upgrade wins.exe
Transfer-File -Src c:\Windows\wins.exe -Dst c:\etc\rancher\wins\wins.exe

Start-Process -NoNewWindow -Wait -FilePath "c:\etc\rancher\agent.exe"
