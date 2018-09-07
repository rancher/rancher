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
## START main definition

$RancherDir = "C:\etc\rancher"
$KubeDir = "C:\etc\kubernetes"
$CNIDir = "C:\etc\cni"
$RancherLogDir = ("C:\var\log\rancher\{0}" -f $(Get-Date -UFormat "%Y%m%d"))

$null = New-Item -Force -Type Directory -Path $RancherDir -ErrorAction Ignore
$null = New-Item -Force -Type Directory -Path $KubeDir -ErrorAction Ignore
$null = New-Item -Force -Type Directory -Path $CNIDir -ErrorAction Ignore

$NodeName = $NodeName.ToLower()
$KubeCNIMode = $KubeCNIMode.ToLower()

Import-Module "$RancherDir\hns.psm1" -Force

function print {
    [System.Console]::Out.Write($args[0])
    Start-Sleep -Milliseconds 100
}

function set-env-var {
    param(
        [parameter(Mandatory = $true)] [string]$Key,
        [parameter(Mandatory = $false)] [string]$Value = ""
    )

    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Process)
    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Machine)
}

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

function merge-argument-list($listArr, $split) {
    if (-not $split) {
        $split = "="
    }

    $mergeRet = @()
    $checkList = @{}
    foreach ($list in $listArr) {
        foreach ($item in $list) {
            $sItem = $item -replace "`"",""
            $sItem = $sItem -split $split
            $sItemKey = $sItem[0]

            if ($checkList[$sItemKey]) {
                continue
            }

            $checkList[$sItemKey] = $True
            $mergeRet += $item
        }
    }

    return $mergeRet -join " "
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
            $ip = ($na | Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Ignore).IPAddress
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

    $subnetMask = (Get-WmiObject Win32_NetworkAdapterConfiguration | ? InterfaceIndex -eq $($na.InterfaceIndex)).IPSubnet[0]
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

function wait-ready {
    param(
        [parameter(Mandatory = $true)] $Path
    )

    $count = 15
    while ((-not (Test-Path $Path)) -and ($count -le 0)) {
        Start-Sleep -s 2
        $count -= 1
    }

    if ($count -le 0) {
        throw ("Timeout and can't access {0}, crash" -f $Path)
    }
}

function reset-proxy {
    try {
        docker restart nginx-proxy *>$null
    } catch {}

    Start-Sleep -s 5
}

function get-pod-cidr() {
    $kubeletArgs = merge-argument-list @(
        @("`"--register-schedulable=false`"")
        @($env:CATTLE_CUSTOMIZE_KUBELET_OPTIONS -split ";")
        @($KubeletOptions -split ";")
    )

    wait-ready -Path "$KubeDir\bin\kubectl.exe"

    pushd $KubeDir\bin
    $podCIDR = ""
    try {
        $podCIDR = (.\kubectl.exe --kubeconfig="$KubeDir\ssl\kubecfg-kube-node.yaml" get node $NodeName -o=jsonpath='{.spec.podCIDR}' 2>$null)
    } catch {}
    if (-not $podCIDR) {
        $retryCount = 7

        if (Test-Path "$env:TEMP\kubelet_temp.xml") {
            $process = Import-Clixml -Path "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
            $process = Get-Process -Id $process.Id -ErrorAction Ignore
            if ($process) {
                $process | Stop-Process | Out-Null
                Remove-Item -Force "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
            }
        }

        wait-ready -Path "$KubeDir\bin\kubelet.exe"

        $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs
        $process | Export-Clixml -Path "$env:TEMP\kubelet_temp.xml" -Force | Out-Null
        while (-not $podCIDR) {
            $process = Get-Process -Id $process.Id -ErrorAction Ignore
            if (-not $process) {
                if ($retryCount -eq 6) {
                    # create an error debug log #
                    $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
                    $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs -RedirectStandardError "$RancherLogDir\detected-kubelet-err.log"
                } else {
                    $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs
                }
                $process | Export-Clixml -Path "$env:TEMP\kubelet_temp.xml" -Force | Out-Null
            }

            print "...................."
            Start-Sleep -s 10

            try {
                $podCIDR = (.\kubectl.exe --kubeconfig="$KubeDir\ssl\kubecfg-kube-node.yaml" get node $NodeName -o=jsonpath='{.spec.podCIDR}' 2>$null)
            } catch {}

            $retryCount -= 1
            if ($retryCount -le 0) {
                break
            }
            if (($retryCount -eq 4) -and (-not $podCIDR)) {
                reset-proxy
            }
        }

        $process | Stop-Process | Out-Null

        # uncordon #
        try {
            .\kubectl.exe --kubeconfig="$KubeDir\ssl\kubecfg-kube-node.yaml" uncordon $NodeName 2>$null | Out-Null
        } catch {}
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
    ## stop stale ##
    try {
        $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process | Out-Null
        }
    } catch {
    }

    ## get pod CIDR ##
    print "Getting Pod CIDR ..."
    $podCIDR = get-pod-cidr
    print "................. OK, $podCIDR"

    $flannelBackendName = ""
    $flannelBackendType = ""
    $flannelNetwork = $KubeClusterCIDR

    if ($KubeCNIMode -eq "overlay") {
        $flannelBackendType = "vxlan"
        $flannelBackendName = "vxlan0"
    } elseif ($KubeCNIMode -eq "l2bridge") {
        $flannelBackendType = "host-gw"
        $flannelBackendName = "cbr0"
    } else {
        throw "Unknown flannel mode: `"win-$KubeCNIMode`", crash"
    }

    # clean other kind network #
    print "Cleaning stale HNSNetwork ..."
    $isCleanPreviousNetwork = Clean-HNSNetworks -Types @{ "l2bridge" = $True; "overlay" = $True } -Keeps @{ $flannelBackendName = $KubeCNIMode }
    if ($isCleanPreviousNetwork) {
        Start-Sleep -s 5
        reset-proxy
    }
    print ".................. OK"

    print "Generating flanneld net-config.json ..."
    $kubeFlannelPath = "C:\etc\kube-flannel"
    $null = New-Item -Force -Type Directory -Path $kubeFlannelPath -ErrorAction Ignore
    $netConfJson = @{
        Network = $flannelNetwork
        Backend = @{
            name = $flannelBackendName
            type = $flannelBackendType
        }
    }
    $netConfJson | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath "$kubeFlannelPath\net-conf.json"
    print ".................................... OK"

    print "Generating cni.conf ..."
    $cniConfPath = "$CNIDir\conf"
    $null = New-Item -Force -Type Directory -Path $cniConfPath -ErrorAction Ignore
    $delegate = $null
    if ($KubeCNIMode -eq "overlay") {
        $delegate = @{
            type = "win-overlay"
            dns = @{
                nameservers = @($KubeDnsServiceIP)
                search = @(
                    "svc." + $KubeClusterDomain
                )
            }
            additionalArgs = @(
                @{
                    name = "EndpointPolicy"
                    value = @{
                        Type = "OutBoundNAT"
                        ExceptionList = @(
                            $KubeClusterCIDR
                            $KubeServiceCIDR
                        )
                    }
                }
                @{
                    name = "EndpointPolicy"
                    value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $KubeServiceCIDR
                    }
                }
            )
        }
    } elseif ($KubeCNIMode -eq "l2bridge") {
        $vswitch = get-hyperv-vswitch

        $delegate = @{
            type = "win-l2bridge"
            dns = @{
                nameservers = @($KubeDnsServiceIP)
                search = @(
                    "svc." + $KubeClusterDomain
                )
            }
            additionalArgs = @(
                @{
                    name = "EndpointPolicy"
                    value = @{
                        Type = "OutBoundNAT"
                        ExceptionList = @(
                            $KubeClusterCIDR
                            $KubeServiceCIDR
                            $vswitch.Subnet.CIDR
                        )
                    }
                }
                @{
                    name = "EndpointPolicy"
                    value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $KubeServiceCIDR
                    }
                }
                @{
                    name = "EndpointPolicy"
                    value = @{
                        Type = "ROUTE"
                        NeedEncap = $true
                        DestinationPrefix = $vswitch.CIDR
                    }
                }
            )
        }
    }

    $cniConf = @{
        cniVersion = "0.2.0"
        name = $flannelBackendName
        type = "flannel"
        capabilities = @{
            dns = $True
        }
        delegate = $delegate
    }
    $cniConf | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath "$cniConfPath\cni.conf"
    print "............................ OK"

    # start flanneld #
    print "Starting flanneld ..."
    wait-ready -Path "$CNIDir\bin\flanneld.exe"

    set-env-var "KUBE_NETWORK" $flannelBackendName
    set-env-var "NODE_NAME" $NodeName
    $flanneldArgs = @(
        "`"--kubeconfig-file=$KubeDir\ssl\kubecfg-kube-node.yaml`""
        "`"--iface=$NodeIP`""
        "`"--ip-masq`""
        "`"--kube-subnet-mgr`""
        "`"--iptables-forward-rules=false`""
    )
    $process = Start-Process -PassThru -FilePath "$CNIDir\bin\flanneld.exe" -ArgumentList $flanneldArgs
    $process | Export-Clixml -Path "$env:TEMP\flanneld.xml" -Force | Out-Null

    Start-Sleep -s 10

    $retryCount = 7
    $process = Get-Process -Id $process.Id -ErrorAction Ignore
    while (-not $process) {
        if ($retryCount -eq 6) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$CNIDir\bin\flanneld.exe" -ArgumentList $flanneldArgs -RedirectStandardError "$RancherLogDir\flanneld-err.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$CNIDir\bin\flanneld.exe" -ArgumentList $flanneldArgs
        }
        $process | Export-Clixml -Path "$env:TEMP\kubelet.xml" -Force | Out-Null

        print "....................."
        Start-Sleep -s 10

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            break
        }
        if ($retryCount -eq 4) {
            reset-proxy
        }
    }

    if (-not $process) {
        throw ".............. FAILED, agent retry"
    }

    $retryCount = 7
    $network = Get-HnsNetwork | ? Name -eq $flannelBackendName
    while(-not $network) {
        print "....................."
        Start-Sleep -s 10

        $network = (Get-HnsNetwork | ? Name -eq $flannelBackendName)

        $retryCount -= 1
        if ($retryCount -le 0) {
            break
        }
    }

    if ((-not $network) -or ($network.Type -ne $KubeCNIMode)) {
        try {
            docker rm -f nginx-proxy *>$null
        } catch {}

        throw ".............. FAILED, agent retry"
    }

    reset-proxy

    print ".................. OK"
}

function start-kubelet {
    ## stop stale ##
    try {
        $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process | Out-Null
        }
    } catch {
    }

    print "Starting kubelet ..."
    wait-ready -Path "$KubeDir\bin\kubelet.exe"

    $kubeletArgs = merge-argument-list @(
        @(
            "`"--register-schedulable=true`""
            "`"--network-plugin=cni`""
            "`"--cni-bin-dir=$CNIDir\bin`""
            "`"--cni-conf-dir=$CNIDir\conf`""
        )
        @($env:CATTLE_CUSTOMIZE_KUBELET_OPTIONS -split ";")
        @($KubeletOptions -split ";")
    )
    $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs
    $process | Export-Clixml -Path "$env:TEMP\kubelet.xml" -Force | Out-Null

    Start-Sleep -s 10

    $retryCount = 7
    $process = Get-Process -Id $process.Id -ErrorAction Ignore
    while (-not $process) {
        if ($retryCount -eq 6) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs -RedirectStandardError "$RancherLogDir\kubelet-err.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs
        }
        $process | Export-Clixml -Path "$env:TEMP\kubelet.xml" -Force | Out-Null

        print "....................."
        Start-Sleep -s 10

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            break
        }
    }

    if (-not $process) {
        throw ".............. FAILED, agent retry"
    }

    reset-proxy

    print "................. OK"

    # uncordon #
    if (Test-Path "$env:TEMP\kubelet_temp.xml") {
        pushd $KubeDir\bin
        try {
            $uncorbonConut = 3
            while ($uncorbonConut -gt 0) {
                .\kubectl.exe --kubeconfig="$KubeDir\ssl\kubecfg-kube-node.yaml" uncordon $NodeName 2>$null | Out-Null
                Start-Sleep -s 2
                $uncorbonConut -= 1
            }
        } catch {}
        popd

        Remove-Item -Force "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
    }
}

function start-kube-proxy {
    ## stop stale ##
    try {
        $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process | Out-Null
        }
    } catch {
    }

    ## broke for Overlay ##
    if ($KubeCNIMode -eq "overlay") {
        Start-Sleep -s 60
    }

    print "Starting kube-proxy ..."
    wait-ready -Path "$KubeDir\bin\kube-proxy.exe"

    $hnsPolicyList = Get-HnsPolicyList
    if ($hnsPolicyList) {
        $hnsPolicyList | Remove-HnsPolicyList
    }

    $env:KUBE_NETWORK = get-env-var "KUBE_NETWORK"

    $kubeproxyArgs = merge-argument-list @(
        @("`"--cluster-cidr=$KubeClusterCIDR`"")
        @($env:CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS -split ";")
        @($KubeproxyOptions -split ";")
    )
    $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs
    $process | Export-Clixml -Path "$env:TEMP\kube-proxy.xml" -Force | Out-Null

    Start-Sleep -s 10

    $retryCount = 7
    $process = Get-Process -Id $process.Id -ErrorAction Ignore
    while (-not $process) {
        if ($retryCount -eq 6) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs -RedirectStandardError "$RancherLogDir\kube-proxy-err.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs
        }
        $process | Export-Clixml -Path "$env:TEMP\kube-proxy.xml" -Force | Out-Null

        print "....................."
        Start-Sleep -s 10

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            break
        }
    }

    if (-not $process) {
        throw ".............. FAILED, agent retry"
    }

    reset-proxy

    print ".................... OK"

    # uncordon #
    if (Test-Path "$env:TEMP\kubelet_temp.xml") {
        pushd $KubeDir\bin
        try {
            $uncorbonConut = 3
            while ($uncorbonConut -gt 0) {
                .\kubectl.exe --kubeconfig="$KubeDir\ssl\kubecfg-kube-node.yaml" uncordon $NodeName 2>$null | Out-Null
                Start-Sleep -s 2
                $uncorbonConut -= 1
            }
        } catch {}
        popd

        Remove-Item -Force "$env:TEMP\kubelet_temp.xml" -ErrorAction Ignore
    }
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
        if (Test-Path "$KubeDir\bin\need_clean.tip") {
            Remove-Item -Force -Path "$KubeDir\bin\*" -ErrorAction Ignore
            $isCleaned = $true
        }
        if (Test-Path "$CNIDir\bin\need_clean.tip") {
            Remove-Item -Force -Path "$CNIDir\bin\*" -ErrorAction Ignore
            $isCleaned = $true
        }
        if ($isCleaned) {
            throw "The previous binaries have already been cleaned, agent retry"
        }

        break
    } else {
        # checking the execution binaries need to be removed or not #
        if ((Test-Path "$KubeDir\bin\need_clean.tip") -or (Test-Path "$CNIDir\bin\need_clean.tip")) {
            $Force = $true
            continue
        }

        # recover processes #
        $shouldUseCompsCnt = 3
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
        $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
        if (-not $process) {
            $wantRecoverComps += @("start-kube-proxy")
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
start-kube-proxy

## END main execution
#########################################################################
