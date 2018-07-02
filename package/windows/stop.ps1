#Requires -Version 5.0

param (
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

function print {
    [System.Console]::Out.Write($args[0])
}

function clean-hnsnetwork {
    param(
        [parameter(Mandatory = $true)] $Network
    )

    docker ps -q | % { docker stop $_ *>$null } *>$null
    # clean up rancher parts #
    docker rm nginx-proxy *>$null
    docker rm kubernetes-binaries *>$null
    docker rm cni-binaries *>$null

    Remove-HNSNetwork $Network
    Start-Sleep -s 10

    print "Clean HNSNetwork"
}

## END main definition
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

# cni #
# flanneld #
try {
    $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process | Out-Null
        print "Stopped flanneld"

        Start-Sleep -s 2
    }
} catch {
    throw "Can't stop the early flanneld process, crash"
}

# kubelet #
try {
    $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process | Out-Null
        print "Stopped kubelet"

        Start-Sleep -s 2
    }
} catch {
    throw "Can't stop the early kubelet process, crash"
}

# kube-proxy #
try {
    $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
    if ($process) {
        $process | Stop-Process | Out-Null
        print "Stopped kube-proxy"

        Start-Sleep -s 2
    }
} catch {
    throw "Can't stop the early kube-proxy process, crash"
}

# network #
Get-HnsNetwork | % {
    $item = $_
    if ($item.Type -eq "l2bridge") {
        clean-hnsnetwork -Network $item
    } elseif ($item.Type -eq "overlay") {
        clean-hnsnetwork -Network $item
    }
}

## END main execution
#########################################################################