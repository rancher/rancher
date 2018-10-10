#Requires -Version 5.0

param (
    [parameter(Mandatory = $true)] [string]$KubeClusterCIDR,
    [parameter(Mandatory = $true)] [string]$KubeClusterDomain,
    [parameter(Mandatory = $true)] [string]$KubeServiceCIDR,
    [parameter(Mandatory = $true)] [string]$KubeDnsServiceIP,
    [parameter(Mandatory = $true)] [string]$KubeCNIComponent,
    [parameter(Mandatory = $true)] [string]$KubeCNIMode,
    [parameter(Mandatory = $false)] [string]$KubeletCloudProviderName,
    [parameter(Mandatory = $false)] [string]$KubeletCloudProviderConfig,
    [parameter(Mandatory = $false)] [string]$KubeletDockerConfig,
    [parameter(Mandatory = $true)] [string]$KubeletOptions,
    [parameter(Mandatory = $true)] [string]$KubeproxyOptions,

    [parameter(Mandatory = $true)] [string]$NodeIP,
    [parameter(Mandatory = $true)] [string]$NodeName
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

function scrape-text {
    param(
        [parameter(Mandatory = $false)] $Headers = @{"Cache-Control"="no-cache"},
        [parameter(Mandatory = $true)] [string]$Uri
    )

    $scraped = Invoke-WebRequest -Headers $Headers -UseBasicParsing -Uri $Uri
    return $scraped.Content
}

function scrape-json {
    param(
        [parameter(Mandatory = $true)] [string]$Uri
    )

    $scraped = Invoke-WebRequest -Headers @{"Accept"="application/json";"Cache-Control"="no-cache"} -UseBasicParsing -Uri $Uri
    return ($scraped.Content | ConvertFrom-Json)
}

function install-msi {
    param(
        [parameter(Mandatory = $true)] [string]$File,
        [parameter(Mandatory = $true)] [string]$LogFile
    )

    $installArgs = @(
        "/i"
        $File
        "/qn"
        "/norestart"
        "/Le"
        $LogFile
    )
    Start-Process "msiexec.exe" -ArgumentList $installArgs -Wait -NoNewWindow
}

function add-routes {
    param(
        [parameter(Mandatory = $true)] [string[]]$IPAddrs
    )

    $vswitch = get-hyperv-vswitch
    foreach ($ipAddr in $IPAddrs) {
        try {
            $null = New-NetRoute -DestinationPrefix $ipAddr -InterfaceIndex $($vswitch.Index) -NextHop $($vswitch.Gateway) -RouteMetric 1 -PolicyStore ActiveStore -ErrorAction Ignore
        } catch {}
    }
}

function repair-cloud-routes {
    switch ($KubeletCloudProviderName) {
        "aws" { add-routes -IPAddrs  @("169.254.169.254/32", "169.254.169.250/32", "169.254.169.251/32") }
        "azure" { add-routes -IPAddrs  @("169.254.169.254/32") }
    }
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
    while ($count -gt 0) {
        Start-Sleep -s 1

        if (Test-Path $Path -ErrorAction Ignore) {
            Start-Sleep -s 5
            break
        }

        Start-Sleep -s 1
        $count -= 1
    }

    if ($count -le 0) {
        throw ("Timeout and can't access {0}, crash" -f $Path)
    }
}

function restart-proxy {
    Start-Sleep -s 5

    try {
        docker restart nginx-proxy *>$null
    } catch {}

    Start-Sleep -s 5
}

function config-cni-flannel {
    param(
        [parameter(Mandatory = $false)] [switch]$Restart = $False
    )

    $flannelBackendName = "vxlan0"
    $flannelBackendType = "vxlan"
    $flannelNetwork = $KubeClusterCIDR
    $networkType = "overlay"
    if ($KubeCNIMode -eq "win-bridge") {
        $flannelBackendType = "host-gw"
        $flannelBackendName = "cbr0"
        $networkType = "l2bridge"
    }

    ## clean other kind network ##
    $isCleanPreviousNetwork = Clean-HNSNetworks -Types @{ "l2bridge" = $True; "overlay" = $True } -Keeps @{ $flannelBackendName = $networkType }
    if ($isCleanPreviousNetwork) {
        print "...................., cleaning stale HNS network"

        restart-proxy
    }

    ## generate flanneld config ##
    $kubeFlannelPath = "C:\etc\kube-flannel"
    if (-not (Test-Path "$kubeFlannelPath\net-conf.json")) {
        print "...................., generating flanneld config"
    } else {
        print "...................., overwriting flanneld config"
    }
    $null = New-Item -Force -Type Directory -Path $kubeFlannelPath -ErrorAction Ignore
    $netConfJson = @{
        Network = $flannelNetwork
        Backend = @{
            name = $flannelBackendName
            type = $flannelBackendType
        }
    }
    $netConfJson | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath "$kubeFlannelPath\net-conf.json"

    ## generate CNI config ##
    $cniConfPath = "$CNIDir\conf"
    if (-not (Test-Path "$cniConfPath\cni.conf")) {
        print "...................., generating cni config"
    } else {
        print "...................., overwriting cni config"
    }
    $null = New-Item -Force -Type Directory -Path $cniConfPath -ErrorAction Ignore
    $delegate = $null
    if ($KubeCNIMode -eq "win-overlay") {
        $delegate = @{
            type = "win-overlay"
            dns = @{
                nameservers = @($KubeDnsServiceIP)
                search = @(
                    "svc." + $KubeClusterDomain
                )
            }
            policies = @(
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
    } elseif ($KubeCNIMode -eq "win-bridge") {
        $vswitch = get-hyperv-vswitch

        $delegate = @{
            type = "win-bridge"
            dns = @{
                nameservers = @($KubeDnsServiceIP)
                search = @(
                    "svc." + $KubeClusterDomain
                )
            }
            policies = @(
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

    set-env-var -Key "KUBE_NETWORK" -Value $flannelBackendName
}

function config-azure-cloudprovider {
    param(
        [parameter(Mandatory = $false)] $ConfigPath = "C:\etc\kubernetes\cloud-config"
    )

    if (-not (Test-Path $ConfigPath)) {
        return
    }

    pushd "C:\Program Files (x86)\Microsoft SDKs\Azure\CLI2\wbin"

    $azureMetaURL = "http://169.254.169.254/metadata/instance/compute"

    $azResourcesGroup = scrape-text -Headers @{"Metadata"="true"} -Uri "$azureMetaURL/resourceGroupName?api-version=2017-08-01&format=text"
    $azSubscriptionId = scrape-text -Headers @{"Metadata"="true"} -Uri "$azureMetaURL/subscriptionId?api-version=2017-08-01&format=text"
    $azLocation = scrape-text -Headers @{"Metadata"="true"} -Uri "$azureMetaURL/location?api-version=2017-08-01&format=text"
    $azVmName = scrape-text -Headers @{"Metadata"="true"} -Uri "$azureMetaURL/name?api-version=2017-08-01&format=text"

    $azCloudConfig = @{}
    try {
        $jsonConfig = cat $ConfigPath | ConvertFrom-Json -ErrorAction Ignore -WarningAction Ignore
        $jsonConfig.PSObject.Properties | % {
            $item = $_
            $azCloudConfig[$item.Name] = $item.Value
        }
    } catch {}
    $azureCloud = $azCloudConfig.cloud
    $azureClientId = $azCloudConfig.aadClientId
    $azureClientSecret = $azCloudConfig.aadClientSecret
    $azureTenantId = $azCloudConfig.tenantId

    if (-not $azureClientId) {
        throw ("Can't find 'aadClientId' in '{0}'" -f $ConfigPath)
    } elseif (-not $azureClientSecret) {
        throw ("Can't find 'aadClientSecret' in '{0}'" -f $ConfigPath)
    } elseif (-not $azureTenantId) {
        throw ("Can't find 'tenantId' in '{0}'" -f $ConfigPath)
    }

    if ((-not $azLocation) -or (-not $azSubscriptionId) -or (-not $azResourcesGroup) -or (-not $azVmName)) {
        print "Some Azure cloud provider variables were not populated correctly, using the passed cloud provider config"
        return
    }

    # setting correct login cloud
    if (-not $azureCloud) {
        $azureCloud = "AzureCloud"
    }
    .\az.cmd cloud set --name $azureCloud 2>$null | Out-Null

    # login to Azure
    .\az.cmd login --service-principal -u $azureClientId -p $azureClientSecret --tenant $azureTenantId 2>$null | Out-Null

    $azVmNic = ($(.\az.cmd vm nic list -g $azResourcesGroup --vm-name $azVmName | ConvertFrom-Json)[0].id -split "/")[8]
    $azVmNicShow = $(.\az.cmd vm nic show -g $azResourcesGroup --vm-name $azVmName --nic $azVmNic) | ConvertFrom-Json
    $azVmNicSubnet = $azVmNicShow.ipConfigurations[0].subnet.id -split "/"
    $azVmNicSecurityGroup = $azVmNicShow.networkSecurityGroup.id -split "/"

    $azSubnetName = $azVmNicSubnet[10]
    $azVnetName = $azVmNicSubnet[8]
    $azVnetResourceGroup = $azVmNicSubnet[4]
    $azVmNsg = $azVmNicSecurityGroup[8]

    # logout from Azure
    .\az.cmd logout 2>$null | Out-Null

    if ((-not $azVnetResourceGroup) -or (-not $azSubnetName) -or (-not $azVnetName) -or (-not $azVmNsg)) {
        print "Some Azure cloud provider variables were not populated correctly, using the passed cloud provider config"
        return
    } else {
        $azCloudConfig["subscriptionId"] = $azSubscriptionId
        $azCloudConfig["location"] = $azLocation
        $azCloudConfig["resourceGroup"] = $azResourcesGroup
        $azCloudConfig["vnetResourceGroup"] = $azVnetResourceGroup
        $azCloudConfig["subnetName"] = $azSubnetName
        $azCloudConfig["useInstanceMetadata"] = $True
        $azCloudConfig["securityGroupName"] = $azVmNsg
        $azCloudConfig["vnetName"] = $azVnetName
        $azCloudConfig | ConvertTo-Json -Compress -Depth 32 | Out-File -Encoding ascii -Force -FilePath $ConfigPath
    }

    popd
}

function stop-flanneld {
    try {
        $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process -Force | Out-Null
        }
    } catch {
    }
}

function start-flanneld {
    param(
        [parameter(Mandatory = $false)] [switch]$Restart = $False
    )

    ## stop stale ##
    stop-flanneld

    if ($Restart) {
        print "Restarting flanneld ."
    } else {
        print "Starting flanneld ..."
    }

    ## binary is ready or not ##
    wait-ready -Path "$CNIDir\bin\flanneld.exe"

    ## config running params ##
    $flanneldArgs = @(
        "`"--kubeconfig-file=$KubeDir\ssl\kubecfg-kube-node.yaml`""
        "`"--iface=$NodeIP`""
        "`"--ip-masq`""
        "`"--kube-subnet-mgr`""
        "`"--iptables-forward-rules=false`""
    )

    ## start and retry ##
    $retryCount = 6
    $process = $null
    while (-not $process) {
        if ($retryCount -eq 3) {
            restart-proxy
        }

        if ($retryCount -eq 1) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$CNIDir\bin\flanneld.exe" -ArgumentList $flanneldArgs -RedirectStandardError "$RancherLogDir\flanneld.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$CNIDir\bin\flanneld.exe" -ArgumentList $flanneldArgs
        }

        print "....................."
        Start-Sleep -s 20

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            if (-not $process) {
                throw ".............. FAILED, agent retry"
            }
            break
        }
    }

    ## check network created or not ##
    print "...................., checking HNS network"
    $flannelBackendName = get-env-var -Key "KUBE_NETWORK"
    $retryCount = 6
    $network = $null
    while(-not $network) {
        $network = (Get-HnsNetwork | ? Name -eq $flannelBackendName)

        print "....................."
        Start-Sleep -s 5

        $retryCount -= 1
        if ($retryCount -le 0) {
            if (-not $network) {
                throw ".............. FAILED, agent retry"
            }
            break
        }
    }

    $networkType = "overlay"
    if ($KubeCNIMode -eq "win-bridge") {
        $networkType = "l2bridge"
    }
    if ($network.Type -ne $networkType) {
        ## restart flanneld
        stop-flanneld
        throw ".............. FAILED, agent retry"
    }

    restart-proxy
    repair-cloud-routes

    print ".................. OK"
}

function stop-kubelet {
    try {
        $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process -Force | Out-Null
        }
    } catch {
    }
}

function start-kubelet {
    param(
        [parameter(Mandatory = $false)] [switch]$Restart = $False
    )

    ## stop stale ##
    stop-kubelet

    if ($Restart) {
        print "Restarting kubelet .."
    } else {
        print "Starting kubelet ...."
    }

    ## binary is ready or not ##
    wait-ready -Path "$KubeDir\bin\kubelet.exe"

    ## config cni ##
    if ($KubeCNIComponent -eq "flannel") {
        config-cni-flannel -Restart:$Restart
    } elseif ($KubeCNIComponent -eq "canal") {
        config-cni-flannel -Restart:$Restart
    } elseif ($KubeCNIComponent -eq "calico") {
        throw "Don't support calico now, please change other CNI plugins, crash"
    } else {
        throw "Unknown CNI component: $KubeCNIComponent, please change other CNI plugins, crash"
    }

    ## cloud provider ##
    if ($KubeletCloudProviderConfig) {
        $configDir = "C:\etc\kubernetes"
        if (-not (Test-Path "$configDir\cloud-config")) {
            print "...................., generating cloudprovider config"
        } else {
            print "...................., overwriting cloudprovider config"
        }

        $null = New-Item -Force -Type Directory -Path $configDir -ErrorAction Ignore
        [System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($KubeletCloudProviderConfig)) | Out-File -Encoding ASCII "$configDir\cloud-config"

        if ($KubeletCloudProviderName -eq "azure") {
            # Azure config patch #
            config-azure-cloudprovider -ConfigPath "$configDir\cloud-config"
        }
    }

    ## docker config ##
    if ($KubeletDockerConfig) {
        $configDir = "C:\var\lib\kubelet"
        if (-not (Test-Path "$configDir\config.json")) {
            print "....................., generating docker config"
        } else {
            print "....................., overwriting docker config"
        }

        $null = New-Item -Force -Type Directory -Path $configDir -ErrorAction Ignore
        [System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($KubeletDockerConfig)) | Out-File -Encoding ASCII "$configDir\config.json"
    }

    ## config running params ##
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

    ## start kubelet ##
    $retryCount = 6
    $process = $null
    while (-not $process) {
        if ($retryCount -eq 1) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs -RedirectStandardError "$RancherLogDir\kubelet.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kubelet.exe" -ArgumentList $kubeletArgs
        }

        print "....................."
        Start-Sleep -s 15

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            if (-not $process) {
                throw ".............. FAILED, agent retry"
            }
            break
        }
    }

    print ".................. OK"

    ## start networking ctrl ##
    if ($KubeCNIComponent -eq "flannel") {
        start-flanneld -Restart:$Restart
    } elseif ($KubeCNIComponent -eq "canal") {
        start-flanneld -Restart:$Restart
    }
}

function stop-kube-proxy {
    try {
        $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
        if ($process) {
            $process | Stop-Process -Force | Out-Null
        }
    } catch {
    }
}

function start-kube-proxy {
    param(
        [parameter(Mandatory = $false)] [switch]$Restart = $False
    )

    ## stop stale ##
    stop-kube-proxy

    if ($Restart) {
        print "Restarting kube-proxy"
    } else {
        print "Starting kube-proxy ."
    }

    ## wait a few seconds ##
    print "...................., wait a few seconds"
    if ($KubeCNIMode -eq "win-overlay") {
        Start-Sleep -s 30
    } else {
        Start-Sleep -s 10
    }

    ## binary is ready or not ##
    wait-ready -Path "$KubeDir\bin\kube-proxy.exe"

    ## clean stale policies ##
    $hnsPolicyList = Get-HnsPolicyList
    if ($hnsPolicyList) {
        print "...................., cleaning stale HNS policies"
        $hnsPolicyList | Remove-HnsPolicyList
    }

    ## config running params ##
    $env:KUBE_NETWORK = get-env-var -Key "KUBE_NETWORK"
    $kubeproxyArgs = merge-argument-list @(
        @("`"--cluster-cidr=$KubeClusterCIDR`"")
        @($env:CATTLE_CUSTOMIZE_KUBEPROXY_OPTIONS -split ";")
        @($KubeproxyOptions -split ";")
    )

    ## start kube-proxy ##
    $retryCount = 6
    $process = $null
    while (-not $process) {
        if ($retryCount -eq 1) {
            # create an error debug log #
            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs -RedirectStandardError "$RancherLogDir\kube-proxy.log"
        } else {
            $process = Start-Process -PassThru -FilePath "$KubeDir\bin\kube-proxy.exe" -ArgumentList $kubeproxyArgs
        }

        print "....................."
        Start-Sleep -s 10

        $process = Get-Process -Id $process.Id -ErrorAction Ignore

        $retryCount -= 1
        if ($retryCount -le 0) {
            if (-not $process) {
                throw ".............. FAILED, agent retry"
            }
            break
        }
    }

    restart-proxy
    print ".................. OK"
}

function init {
    # stale binaries clean #
    $removeStaleBinaries = $false
    if (Test-Path "$KubeDir\bin\need_clean.tip") {
        stop-kube-proxy
        stop-kubelet

        Remove-Item -Force -Recurse -Path "$KubeDir\bin\*" -ErrorAction Ignore
        $removeStaleBinaries = $true
    }
    if (Test-Path "$CNIDir\bin\need_clean.tip") {
        stop-flanneld

        Remove-Item -Force -Recurse -Path "$CNIDir\bin\*" -ErrorAction Ignore
        $removeStaleBinaries = $true
    }
    if ($removeStaleBinaries) {
        throw "The previous binaries have already been cleaned, agent retry"
    }

    # cloud provider #
    if ($KubeletCloudProviderName -eq "azure") {
        ## verify az cli is installed or not
        $azBinPath = "C:\Program Files (x86)\Microsoft SDKs\Azure\CLI2\wbin\az.cmd"
        if (-not (Test-Path $azBinPath)) {
            print "Can't find Azure cloud cli on this host, try to download ..."

            $azDownloadURL = "https://aka.ms/installazurecliwindows"
            $azMSIBinPath = "$env:TEMP\az.msi"
            try {
                Invoke-WebRequest -TimeoutSec 300 -UseBasicParsing -Uri $azDownloadURL -OutFile $azMSIBinPath
            } catch {}
            if (-not $?) {
                throw ("Failed to download Azure cloud cli from '{0}', crash" -f $azDownloadURL)
            }

            print "Installing Azure cloud cli, wait a few minutes ..."

            $null = New-Item -Force -Type Directory -Path $RancherLogDir -ErrorAction Ignore
            install-msi -File $azMSIBinPath -LogFile "$RancherLogDir\azurecli-installation.log"
            if (-not $?) {
                throw "Failed to install Azure cloud cli, crash"
            }

            print ".................. OK"
        }
    } elseif ($KubeletCloudProviderName -eq "aws") {
        repair-cloud-routes
        ## using private DNS name
        $NodeName = scrape-text -Uri "http://169.254.169.254/latest/meta-data/hostname"
    }
    set-env-var -Key "NODE_NAME" -Value $NodeName
}

function main {
    # recover processes #
    $shouldUseCompsCnt = 3
    $wantRecoverComps = @()

    # kubelet #
    $process = Get-Process -Name "kubelet*" -ErrorAction Ignore
    if (-not $process) {
        $wantRecoverComps += @("kubelet")
    }

    # networking ctrl #
    if ($KubeCNIComponent -eq "flannel") {
        # flanneld #
        $process = Get-Process -Name "flanneld*" -ErrorAction Ignore
        if (-not $process) {
            $wantRecoverComps += @("flanneld")
        }
    }

    # kube-proxy #
    $process = Get-Process -Name "kube-proxy*" -ErrorAction Ignore
    if (-not $process) {
        $wantRecoverComps += @("kube-proxy")
    }

    if ($wantRecoverComps.Count -ne $shouldUseCompsCnt) {
        $recoverKubelet = $False
        $wantRecoverComps | % {
            switch ($_) {
                "kubelet$" {
                    $recoverKubelet = $True
                    start-kubelet -Restart
                    break
                }
                "flanneld" {
                    if (-not $recoverKubelet) {
                        start-flanneld -Restart
                    }
                }
                "kube-proxy" {
                    start-kube-proxy -Restart
                    break
                }
            }
        }
    } else {
        # start kubelet #
        start-kubelet

        # start kube-proxy #
        start-kube-proxy
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
        restart-proxy

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

init

main

## END main execution
#########################################################################
