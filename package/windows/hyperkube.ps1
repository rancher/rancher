#Requires -Version 5.0

param (
    [parameter(Mandatory = $true)] [string]$KubeClusterCIDR,
    [parameter(Mandatory = $true)] [string]$KubeClusterDomain,
    [parameter(Mandatory = $true)] [string]$KubeServiceCIDR,
    [parameter(Mandatory = $true)] [string]$KubeDnsServiceIP,
    [parameter(Mandatory = $true)] [string]$KubeCNIComponent,
    [parameter(Mandatory = $true)] [string]$KubeCNIMode,
    [parameter(Mandatory = $true)] [string]$KubeletOptions,
    [parameter(Mandatory = $true)] [string]$KubeproxyOptions,

    [parameter(Mandatory = $true)] [string]$NodeIP,
    [parameter(Mandatory = $true)] [string]$NodeName,

    [parameter(Mandatory = $false)] [switch]$Force = $false
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

#########################################################################
## START hNS management

function Get-VmComputeNativeMethods()
{
    $signature = @'
                 [DllImport("vmcompute.dll")]
                 public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@

    # Compile into runtime type
    try {
        Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name NativeMethods -PassThru -ErrorAction Ignore
    } catch {}
}

function Get-HnsSwitchExtensions
{
    param
    (
        [parameter(Mandatory=$true)] [string] $NetworkId
    )

    return (Get-HNSNetwork $NetworkId).Extensions
}

function Set-HnsSwitchExtension
{
    param
    (
        [parameter(Mandatory=$true)] [string] $NetworkId,
        [parameter(Mandatory=$true)] [string] $ExtensionId,
        [parameter(Mandatory=$true)] [bool]   $state
    )

    # { "Extensions": [ { "Id": "...", "IsEnabled": true|false } ] }
    $req = @{
        "Extensions"=@(
            @{
                "Id"=$ExtensionId
                "IsEnabled"=$state
            }
        )
    }
    Invoke-HNSRequest -Method POST -Type networks -Id $NetworkId -Data (ConvertTo-Json $req)
}

function Get-HNSActivities
{
    [cmdletbinding()]Param()
    return Invoke-HNSRequest -Type activities -Method GET
}

function Get-HNSPolicyList {
    [cmdletbinding()]Param()
    return Invoke-HNSRequest -Type policylists -Method GET
}

function Remove-HnsPolicyList
{
    [CmdletBinding()]
    param
    (
        [parameter(Mandatory=$true,ValueFromPipeline=$True,ValueFromPipelinebyPropertyName=$True)]
        [Object[]] $InputObjects
    )
    begin {$Objects = @()}
    process {$Objects += $InputObjects; }
    end {
        $Objects | foreach {  Invoke-HNSRequest -Method DELETE -Type  policylists -Id $_.Id }
    }
}

function New-HnsRoute {
    param
    (
        [parameter(Mandatory = $false)] [Guid[]] $Endpoints = $null,
        [parameter(Mandatory = $true)] [string] $DestinationPrefix,
        [parameter(Mandatory = $false)] [switch] $EncapEnabled
    )

    $policyLists = @{
        References = @(
                get-endpointReferences $Endpoints
        )
        Policies   = @(
            @{
                Type = "ROUTE";
                DestinationPrefix = $DestinationPrefix
                NeedEncap = $EncapEnabled.IsPresent
            }
        )
    }

    Invoke-HNSRequest -Method POST -Type policylists -Data (ConvertTo-Json  $policyLists -Depth 10)
}

function New-HnsLoadBalancer {
    param
    (
        [parameter(Mandatory = $false)] [Guid[]] $Endpoints = $null,
        [parameter(Mandatory = $true)] [int] $InternalPort,
        [parameter(Mandatory = $true)] [int] $ExternalPort,
        [parameter(Mandatory = $false)] [string] $Vip
    )

    $policyLists = @{
        References = @(
            get-endpointReferences $Endpoints
        )
        Policies   = @(
            @{
                Type = "ELB"
                InternalPort = $InternalPort
                ExternalPort = $ExternalPort
                VIPs = @($Vip)
            }
        );
    }

    Invoke-HNSRequest -Method POST -Type policylists -Data ( ConvertTo-Json  $policyLists -Depth 10)
}

function get-endpointReferences {
    param
    (
        [parameter(Mandatory = $true)] [Guid[]] $Endpoints = $null
    )
    if ($Endpoints ) {
        $endpointReference = @()
        foreach ($endpoint in $Endpoints)
        {
            $endpointReference += "/endpoints/$endpoint"
        }
        return $endpointReference
    }
    return @()
}

function Get-HNSNetwork
{
    param
    (
        [parameter(Mandatory=$false)] [string] $Id = [Guid]::Empty
    )

    if ($Id -ne [Guid]::Empty)
    {
        return Invoke-HNSRequest -Method GET -Type networks -Id $id
    }
    else
    {
        return Invoke-HNSRequest -Method GET -Type networks
    }
}

function Remove-HNSNetwork
{
    [CmdletBinding()]
    param
    (
        [parameter(Mandatory=$true,ValueFromPipeline=$True,ValueFromPipelinebyPropertyName=$True)]
        [Object[]] $InputObjects
    )
    begin {$Objects = @()}
    process {$Objects += $InputObjects; }
    end {
        $Objects | foreach {  Invoke-HNSRequest -Method DELETE -Type  networks -Id $_.Id }
    }
}

function New-HnsNetwork
{
    param
    (
        [parameter(Mandatory=$false, Position=0)]
        [string] $JsonString,
        [ValidateSet('ICS', 'Internal', 'Transparent', 'NAT', 'Overlay', 'L2Bridge', 'L2Tunnel', 'Layered', 'Private')]
        [parameter(Mandatory = $false, Position = 0)]
        [string] $Type,
        [parameter(Mandatory = $false)] [string] $Name,
        [parameter(Mandatory = $false)] [string] $AddressPrefix,
        [parameter(Mandatory = $false)] [string] $Gateway,
        [parameter(Mandatory = $false)] [string] $DNSServer,
        [parameter(Mandatory = $false)] [string] $AdapterName
    )

    Begin {
        if (!$JsonString) {
            $netobj = @{
                Type          = $Type;
            };

            if ($Name) {
                $netobj += @{
                    Name = $Name;
                }
            }

            if ($AddressPrefix -and  $Gateway) {
                $netobj += @{
                    Subnets = @(
                    @{
                        AddressPrefix  = $AddressPrefix;
                        GatewayAddress = $Gateway;
                    }
                    );
                }
            }

            if ($DNSServerName) {
                $netobj += @{
                    DNSServerList = $DNSServer
                }
            }

            if ($AdapterName) {
                $netobj += @{
                    NetworkAdapterName = $AdapterName;
                }
            }

            $JsonString = ConvertTo-Json $netobj -Depth 10
        }

    }
    Process{
        return Invoke-HNSRequest -Method POST -Type networks -Data $JsonString
    }
}

function Get-HnsEndpoint
{
    param
    (
        [parameter(Mandatory=$false)] [string] $Id = [Guid]::Empty
    )

    if ($Id -ne [Guid]::Empty)
    {
        return Invoke-HNSRequest -Method GET -Type endpoints -Id $id
    }
    else
    {
        return Invoke-HNSRequest -Method GET -Type endpoints
    }
}

function Remove-HNSEndpoint
{
    param
    (
        [parameter(Mandatory = $true, ValueFromPipeline = $True, ValueFromPipelinebyPropertyName = $True)]
        [Object[]] $InputObjects
    )

    begin {$objects = @()}
    process {$Objects += $InputObjects; }
    end {
        $Objects | foreach {  Invoke-HNSRequest -Method DELETE -Type endpoints -Id $_.Id  }
    }
}

function New-HnsEndpoint
{
    param
    (
        [parameter(Mandatory=$false, Position = 0)] [string] $JsonString = $null,
        [parameter(Mandatory = $false, Position = 0)] [Guid] $NetworkId,
        [parameter(Mandatory = $false)] [string] $Name,
        [parameter(Mandatory = $false)] [string] $IPAddress,
        [parameter(Mandatory = $false)] [string] $Gateway,
        [parameter(Mandatory = $false)] [string] $MacAddress,
        [parameter(Mandatory = $false)] [switch] $EnableOutboundNat
    )

    begin
    {
        if ($JsonString)
        {
            $EndpointData = $JsonString | ConvertTo-Json | ConvertFrom-Json
        }
        else
        {
            $endpoint = @{
                VirtualNetwork = $NetworkId;
                Policies       = @();
            }

            if ($Name) {
                $endpoint += @{
                    Name = $Name;
                }
            }

            if ($MacAddress) {
                $endpoint += @{
                    MacAddress     = $MacAddress;
                }
            }

            if ($IPAddress) {
                $endpoint += @{
                    IPAddress      = $IPAddress;
                }
            }

            if ($Gateway) {
                $endpoint += @{
                    GatewayAddress = $Gateway;
                }
            }

            if ($EnableOutboundNat) {
                $endpoint.Policies += @{
                    Type = "OutBoundNAT";
                }

            }
            # Try to Generate the data
            $EndpointData = convertto-json $endpoint
        }
    }

    Process
    {
        return Invoke-HNSRequest -Method POST -Type endpoints -Data $EndpointData
    }
}


function New-HnsRemoteEndpoint
{
    param
    (
        [parameter(Mandatory = $true)] [Guid] $NetworkId,
        [parameter(Mandatory = $false)] [string] $IPAddress,
        [parameter(Mandatory = $false)] [string] $MacAddress
    )

    $remoteEndpoint = @{
        ID = [Guid]::NewGuid();
        VirtualNetwork = $NetworkId;
        IPAddress = $IPAddress;
        MacAddress = $MacAddress;
        IsRemoteEndpoint = $true;
    }

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $remoteEndpoint  -Depth 10)

}


function Attach-HnsHostEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [int] $CompartmentID
    )
    $request = @{
        SystemType    = "Host";
        CompartmentId = $CompartmentID;
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request) -Action "attach" -Id $EndpointID
}

function Attach-HNSVMEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [string] $VMNetworkAdapterName
    )

    $request = @{
        VirtualNicName   = $VMNetworkAdapterName;
        SystemType    = "VirtualMachine";
    };
    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "attach" -Id $EndpointID

}

function Attach-HNSEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [int] $CompartmentID,
        [parameter(Mandatory=$true)] [string] $ContainerID
    )
    $request = @{
        ContainerId = $ContainerID;
        SystemType="Container";
        CompartmentId = $CompartmentID;
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request) -Action "attach" -Id $EndpointID
}

function Detach-HNSVMEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID
    )
    $request = @{
        SystemType  = "VirtualMachine";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}

function Detach-HNSHostEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID
    )
    $request = @{
        SystemType  = "Host";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}

function Detach-HNSEndpoint
{
    param
    (
        [parameter(Mandatory=$true)] [Guid] $EndpointID,
        [parameter(Mandatory=$true)] [string] $ContainerID
    )

    $request = @{
        ContainerId = $ContainerID;
        SystemType="Container";
    };

    return Invoke-HNSRequest -Method POST -Type endpoints -Data (ConvertTo-Json $request ) -Action "detach" -Id $EndpointID
}

function Invoke-HNSRequest
{
    param
    (
        [ValidateSet('GET', 'POST', 'DELETE')]
        [parameter(Mandatory=$true)] [string] $Method,
        [ValidateSet('networks', 'endpoints', 'activities', 'policylists', 'endpointstats', 'plugins')]
        [parameter(Mandatory=$true)] [string] $Type,
        [parameter(Mandatory=$false)] [string] $Action = $null,
        [parameter(Mandatory=$false)] [string] $Data = $null,
        [parameter(Mandatory=$false)] [Guid] $Id = [Guid]::Empty
    )

    $hnsPath = "/$Type"

    if ($id -ne [Guid]::Empty)
    {
        $hnsPath += "/$id";
    }

    if ($Action)
    {
        $hnsPath += "/$Action";
    }

    $request = "";
    if ($Data)
    {
        $request = $Data
    }

    $output = "";
    $response = "";

    $hnsApi = Get-VmComputeNativeMethods
    $hnsApi::HNSCall($Method, $hnsPath, "$request", [ref] $response);

    if ($response)
    {
        try {
            $output = ($response | ConvertFrom-Json);
        } catch {
            Write-Error $_.Exception.Message
            return ""
        }
        if ($output.Error)
        {
            Write-Error $output;
        }
        $output = $output.Output;
    }

    return $output;
}

## END hNS management
#########################################################################
## START main definition

$rancherDir = "C:\etc\rancher"
$kubeDir = "C:\etc\kubernetes"
$cniDir = "C:\etc\cni"
$env:NODE_NAME = $NodeName.ToLower()
$KubeCNIMode = $KubeCNIMode.ToLower()

function print {
    [System.Console]::Out.Write($args[0])
}

function convert-to-decimal-ip {
    param(
        [Parameter(Mandatory = $true, Position = 0)]
        [Net.IPAddress] $ipAddress
    )

    $i = 3
    $decimalIP = 0

    $ipAddress.GetAddressBytes() | % {
        $decimalIP += $_ * [Math]::Pow(256, $i)
        $i--
    }

    return [UInt32]$decimalIP
}

function convert-to-dotted-ip {
    param(
        [Parameter(Mandatory = $true, Position = 0)]
        [Uint32] $ipAddress
    )

    $dottedIP = $(for ($i = 3; $i -gt -1; $i--) {
        $base = [Math]::Pow(256, $i)
        $remainder = $ipAddress % $base
        ($ipAddress - $remainder) / $base
        $ipAddress = $remainder
    })

    return [String]::Join(".", $dottedIP)
}

function convert-to-mask-length {
    param(
        [Parameter(Mandatory = $true, Position = 0)]
        [Net.IPAddress] $subnetMask
    )

    $bits = "$($subnetMask.GetAddressBytes() | % {
        [Convert]::ToString($_, 2)
    } )" -replace "[\s0]"

    return $bits.Length
}

function get-hyperv-vswitch {
    $na = $null
    $ip = $null

    foreach ($nai in Get-NetAdapter) {
        try {
            $na = $nai
            $ip = ($na | Get-NetIPAddress -AddressFamily IPv4).IPAddress
            if ($ip -eq $NodeIP) {
                break
            }
        } catch {}
    }
    if (-not $na) {
        $na = Get-NetAdapter | ? Name -like "vEthernet (Ethernet*"
        if (-not $na) {
            throw "Failed to find a suitable Hyper-V vSwitch network adapter, check your network settings, crash"
        }
        $ip = (Get-NetIPAddress -InterfaceIndex $na.ifIndex -AddressFamily IPv4).IPAddress
    }

    $subnetMask = (Get-WmiObject Win32_NetworkAdapterConfiguration | ? InterfaceIndex -eq $($na.ifIndex)).IPSubnet[0]
    $subnet = (convert-to-decimal-ip $ip) -band (convert-to-decimal-ip $subnetMask)
    $subnet = convert-to-dotted-ip $subnet
    $subnetCIDR = "$subnet/$(convert-to-mask-length $subnetMask)"
    $gw = (Get-NetRoute -InterfaceIndex $na.ifIndex -DestinationPrefix "0.0.0.0/0").NextHop

    return @{
        Name = $na.ifAlias
        Index = $na.ifIndex
        IP = $ip
        CIDR = "$ip/32"
        Subnet = @{
            IP = $subnet
            Mask = $subnetMask
            CIDR = $subnetCIDR
        }
        Gateway = $gw
    }
}

function clean-hnsnetwork {
    param(
        [parameter(Mandatory = $true)] $Network
    )

    docker ps -q | % { docker stop $_ *>$null } *>$null

    print "Cleaning up the early HNS network"
    Remove-HNSNetwork $Network
    Start-Sleep -s 10

    try {
        docker restart nginx-proxy *>$null
        Start-Sleep -s 5
    } catch {}
}

function wait-ready {
    param(
        [parameter(Mandatory = $true)] $Path
    )

    while (-not (Test-Path $Path)) {
        Start-Sleep -s 2
    }
}

function get-pod-cidr() {
    $kubeletOptions = @($KubeletOptions -split ";")

    wait-ready -Path "$kubeDir\bin\kubectl.exe"

    pushd $kubeDir\bin
    try {
        $podCIDR = (.\kubectl.exe --kubeconfig="$kubeDir\ssl\kubecfg-kube-node.yaml" get nodes/$($env:NODE_NAME) -o custom-columns=podCIDR:.spec.podCIDR --no-headers 2>$null)
    } catch {}
    if (-not $podCIDR) {
        $retryCount = 7

        if (Test-Path "$env:TEMP\kubelet_temp.xml") {
            $process = Import-Clixml -Path "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
            $process = Get-Process -Id $process.Id -ErrorAction Ignore
            if ($process) {
                $process | Stop-Process | Out-Null
            }
            Remove-Item -Force "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
        }

        wait-ready -Path "$kubeDir\bin\kubelet.exe"

        $process = Start-Process -PassThru -FilePath "$kubeDir\bin\kubelet.exe" -ArgumentList $kubeletOptions
        $process | Export-Clixml -Path "$env:TEMP\kubelet_temp.xml" -Force | Out-Null
        while (-not $podCIDR) {
            $process = Get-Process -Id $process.Id -ErrorAction Ignore
            if (-not $process) {
                $process = Start-Process -PassThru -FilePath "$kubeDir\bin\kubelet.exe" -ArgumentList $kubeletOptions
                $process | Export-Clixml -Path "$env:TEMP\kubelet_temp.xml" -Force | Out-Null
            }

            print "...................."
            Start-Sleep -s 10

            try {
                $podCIDR = (.\kubectl.exe --kubeconfig="$kubeDir\ssl\kubecfg-kube-node.yaml" get nodes/$($env:NODE_NAME) -o custom-columns=podCIDR:.spec.podCIDR --no-headers 2>$null)
            } catch {}

            $retryCount -= 1
            if ($retryCount -le 0) {
                break
            }
        }

        $process | Stop-Process | Out-Null
        Remove-Item -Force "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
    }
    popd

    if (-not $podCIDR) {
        try {
            docker rm -f nginx-proxy *>$null
        } catch {}
        throw "............. FAILED, agent retry"
    }

    return $podCIDR
}

function config-cni-flannel {
    if (($KubeCNIMode -ne "overlay") -and ($KubeCNIMode -ne "l2bridge")) {
        throw "Unknown flannel mode: `"win-$KubeCNIMode`", crash"
    }

    Get-HnsNetwork | % {
        $item = $_
        if ($item.Type -eq "l2bridge") {
            clean-hnsnetwork -Network $item
        } elseif ($item.Type -eq "overlay") {
            clean-hnsnetwork -Network $item
        }
    }

    ## get pod CIDR ##
    print "Getting Pod CIDR ..."
    $podCIDR = get-pod-cidr
    print "................. OK, $podCIDR"

    # open firewall #
    print "Checking Firewall ..."
    $overlayRule = Get-NetFirewallRule -DisplayName 'Overlay Network Traffic (UDP)' -ErrorAction Ignore
    if ($overlayRule) {
        Set-NetFirewallRule -DisplayName 'Overlay Network Traffic (UDP)' -LocalPort 4789 -Protocol UDP -Profile Any -Action Allow -Enabled True -ErrorAction Ignore | Out-Null
    } else {
        $null = New-NetFirewallRule  -Group 'Rancher' -Description 'Overlay network traffic for Flannel. [UDP 4789]' -Name 'OVERLAY-NETWORK-TRAFFIC-UDP-ANY' -DisplayName 'Overlay Network Traffic (UDP)' -LocalPort 4789 -Protocol UDP -Profile Any -Action Allow -Enabled True
    }
    print ".................. OK"

    print "Generating flannel cni.conf ..."
    $cniConfPath = "$cniDir\conf"
    $null = New-Item -Force -Type Directory -Path $cniConfPath -ErrorAction Ignore
    $cniConf = $null
    if ($KubeCNIMode -eq "overlay") {
        $cniConf = @{
            cniVersion = "0.2.0"
            name = "vxlan0"
            type = "flannel"
            delegate = @{
                type = "win-overlay"
                dns = @{
                    Nameservers = @($KubeDnsServiceIP)
                    Search = @("svc." + $KubeClusterDomain)
                }
                AdditionalArgs = @(
                @{
                    Name = "EndpointPolicy"
                    Value = @{
                        Type = "OutBoundNAT"
                        ExceptionList = @(
                        $KubeClusterCIDR
                        $KubeServiceCIDR
                        )
                    }
                }
                @{
                    Name = "EndpointPolicy"
                    Value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $KubeServiceCIDR
                    }
                }
                )
            }
        }
    } elseif ($KubeCNIMode -eq "l2bridge") {
        $vswitch = get-hyperv-vswitch
        $cniConf = @{
            cniVersion = "0.2.0"
            name = "cbr0"
            type = "flannel"
            delegate = @{
                type = "win-l2bridge"
                dns = @{
                    Nameservers = @($KubeDnsServiceIP)
                    Search = @("svc." + $KubeClusterDomain)
                }
                AdditionalArgs = @(
                @{
                    Name = "EndpointPolicy"
                    Value = @{
                        Type = "OutBoundNAT"
                        ExceptionList = @(
                        $KubeClusterCIDR
                        $KubeServiceCIDR
                        $vswitch.Subnet.CIDR
                        )
                    }
                }
                @{
                    Name = "EndpointPolicy"
                    Value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $KubeServiceCIDR
                    }
                }
                @{
                    Name = "EndpointPolicy"
                    Value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $vswitch.CIDR
                    }
                }
                )
            }
        }
    }
    $cniConf | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath "$cniConfPath\cni.conf"
    print "............................ OK"

    print "Generating flanneld net-config.json ..."
    $kubeFlannelPath = "C:\etc\kube-flannel"
    $null = New-Item -Force -Type Directory -Path $kubeFlannelPath -ErrorAction Ignore
    $netConfJson = $null
    if ($KubeCNIMode -eq "overlay") {
        $env:KUBE_NETWORK = "vxlan0"
        $netConfJson = @{
            Network = $podCIDR
            Backend = @{
                name = "vxlan0"
                type = "vxlan"
            }
        }
    } elseif ($KubeCNIMode -eq "l2bridge") {
        $env:KUBE_NETWORK = "cbr0"
        $netConfJson = @{
            Network = $podCIDR
            Backend = @{
                name = "cbr0"
                type = "host-gw"
            }
        }
    }
    $netConfJson | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath "$kubeFlannelPath\net-conf.json"
    $hnsPolicyList = Get-HnsPolicyList
    if (-not $hnsPolicyList) {
        $hnsPolicyList | Remove-HnsPolicyList
    }
    print ".................................... OK"

    wait-ready -Path "$cniDir\bin\flanneld.exe"

    # start flanneld #
    print "Starting flanneld ..."
    $flanneldArgs = @(
    "--kubeconfig-file=`"$kubeDir\ssl\kubecfg-kube-node.yaml`""
    "--iface=$NodeIP"
    "--ip-masq"
    "--kube-subnet-mgr"
    )
    $process = Start-Process -PassThru -FilePath "$cniDir\bin\flanneld.exe" -ArgumentList $flanneldArgs
    $process | Export-Clixml -Path "$env:TEMP\flanneld.xml" -Force | Out-Null

    $network = Get-HnsNetwork | ? Type -eq $KubeCNIMode
    if (-not $network) {
        $retryCount = 7

        while(-not $network) {
            $process = Get-Process -Id $process.Id -ErrorAction Ignore
            if (-not $process) {
                $process = Start-Process -PassThru -FilePath "$cniDir\bin\flanneld.exe" -ArgumentList $flanneldArgs
                $process | Export-Clixml -Path "$env:TEMP\flanneld.xml" -Force | Out-Null
            }

            print "....................."
            Start-Sleep -s 10

            $network = (Get-HnsNetwork | ? Type -eq $KubeCNIMode)

            $retryCount -= 1
            if ($retryCount -le 0) {
                break
            }
        }
    }

    if (-not $network) {
        try {
            docker rm -f nginx-proxy *>$null
        } catch {}

        throw ".............. FAILED, agent retry"
    } else {
        try {
            Start-Sleep -s 10
            docker restart nginx-proxy *>$null
        } catch {}

        print ".................. OK"
    }
}

function start-kubelet {
    print "Starting kubelet ..."

    try {
        $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process | Out-Null
        }
    } catch {
    }

    $kubeletCustomOptions = @()
    if ($env:CATTLE_CUSTOMIZE_KUBELET_OPTIONS) {
        $kubeletCustomOptions = @($env:CATTLE_CUSTOMIZE_KUBELET_OPTIONS -split ";")
    }
    $kubeletOptions = $kubeletCustomOptions += @($KubeletOptions -split ";")

    $kubeletArgs = $kubeletOptions += @(
    "--network-plugin=cni"
    "--cni-bin-dir=`"$cniDir\bin`""
    "--cni-conf-dir=`"$cniDir\conf`""
    )
    Start-Process -PassThru -FilePath "$kubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs | Export-Clixml -Path "$env:TEMP\kubelet.xml" -Force | Out-Null

    print "................. OK"
}

function start-kube-proxy {
    print "Starting kube-proxy ..."

    $kubeproxyCustomOptions = @()
    if ($env:CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS) {
        $kubeproxyCustomOptions = @($env:CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS -split ";")
    }
    $kubeproxyOptions = @($KubeproxyOptions -split ";")

    $kubeproxyArgs = $kubeproxyCustomOptions += $kubeproxyOptions
    Start-Process -PassThru -FilePath "$kubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs | Export-Clixml -Path "$env:TEMP\kube-proxy.xml" -Force | Out-Null

    print ".................... OK"
}

## END main definitaion
#########################################################################
## START main execution

# 0 - success
# 1 - crash
# 2 - agent retry
trap {
    $errMsg = $_.Exception.Message

    popd

    [System.Console]::Error.Write($errMsg)

    if ($errMsg.EndsWith("agent retry")) {
        exit 2
    }

    exit 1
}

# check powershell #
if ($PSVersionTable.PSVersion.Major -lt 5) {
    throw "PowerShell version 5 or higher is required, crash"
}

# check identity #
$currentPrincipal = new-object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option, crash"
}

while ($true) {
    if ($Force) {
        # clean processes #
        # cni #
        if ($KubeCNIComponent -eq "flannel") {
            # flanneld #
            try {
                $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
                if ($process) {
                    $process | Stop-Process | Out-Null
                }
            } catch {
                throw "Can't stop the early flanneld process, please stop it manually, crash"
            }
        }

        # kubelet #
        try {
            $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
            if ($process) {
                $process | Stop-Process | Out-Null
            }
        } catch {
            throw "Can't stop the early kubelet process, please stop it manually, crash"
        }

        # kube-proxy #
        try {
            $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
            if ($process) {
                $process | Stop-Process | Out-Null
            }
        } catch {
            throw "Can't stop the early kube-proxy process, please stop it manually, crash"
        }

        # checking the execution binaries need to be removed or not #
        $isCleaned = $false
        if (Test-Path "$kubeDir\bin\need_clean.tip") {
            Remove-Item -Force -Path "$kubeDir\bin\*" -ErrorAction Ignore
            $isCleaned = $true
        }
        if (Test-Path "$cniDir\bin\need_clean.tip") {
            Remove-Item -Force -Path "$cniDir\bin\*" -ErrorAction Ignore
            $isCleaned = $true
        }
        if ($isCleaned) {
            throw "The previous binaries have already been cleaned, agent retry"
        }

        break
    } else {
        # checking the execution binaries need to be removed or not #
        if ((Test-Path "$kubeDir\bin\need_clean.tip") -or (Test-Path "$cniDir\bin\need_clean.tip")) {
            $Force = $true
            continue
        }

        # recover processes #
        $shouldUseCompsCnt = 2
        $wantRecoverComps = @()

        # cni #
        if ($KubeCNIComponent -eq "flannel") {
            # flanneld #
            $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
            if (-not $process) {
                $wantRecoverComps += @("config-cni-flannel")
            }
        }

        # kubelet #
        $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
        if (-not $process) {
            $wantRecoverComps += @("start-kubelet")
        }

        # kube-proxy #
        # kube-proxy currently is meant fot the l2bridge only #
        if (-not (($KubeCNIComponent -eq "flannel") -and ($KubeCNIMode -eq "overlay"))) {
            $shouldUseCompsCnt += 1

            $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
            if (-not $process) {
                $wantRecoverComps += @("start-kube-proxy")
            }
        }

        if ($wantRecoverComps.Count -ne $shouldUseCompsCnt) {
            $wantRecoverComps | % {
                switch ($_) {
                    "config-cni-flannel" { config-cni-flannel; break }
                    "start-kubelet" { start-kubelet; break }
                    "start-kube-proxy" { start-kube-proxy }
                }
            }
            exit 0
        } else {
            $Force = $true
        }
    }
}

# config kubernetes cni #
if ($KubeCNIComponent -eq "flannel") {
    config-cni-flannel
} elseif ($KubeCNIComponent -eq "canal") {
    config-cni-flannel
} elseif ($KubeCNIComponent -eq "calico") {
    throw "Don't support calico now, please change other CNI plugins, crash"
} else {
    throw "Unknown CNI component: $KubeCNIComponent, please change other CNI plugins, crash"
}

# start kubelet #
start-kubelet

# start kube-proxy #
# kube-proxy currently is meant fot the l2bridge only #
if (($KubeCNIComponent -eq "flannel") -and ($KubeCNIMode -eq "overlay")) {
    exit 0
}
Start-Sleep -s 15
start-kube-proxy

## END main execution
#########################################################################
