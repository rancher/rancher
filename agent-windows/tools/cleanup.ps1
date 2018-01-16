Import-Module -Name ./rancher-tools.psm1 -Verbose

Stop-Service rancher-agent
Stop-Service rancher-per-host-subnet

$routerIpKey="io.rancher.network.per_host_subnet.router_ip"
$SubnetKey="io.rancher.network.per_host_subnet.subnet"
$defaultNetworkName="transparent"
$routerIp=Get-RancherLabel "$routerIpKey"
$subnet=Get-RancherLabel "$SubnetKey"
if("$subnet" -ne ""){
    if($(Test-RancherNetwork "$subnet")){
        $output=docker network rm $defaultNetworkName 2>$null
        if("$output" -eq "$defaultNetworkName"){
            Write-Host "Clean up transaprent success"
        }
    }
}

## reset nat
netsh routing ip nat uninstall > $null
netsh routing ip nat install > $null

## remove net device
& "c:\program files\rancher\devcon.exe" remove *MSLOOP > $null

## remove net route
Get-NetIPAddress -IPAddress "$routerIp" | get-netroute | Where-Object {$_.NextHop -ne "0.0.0.0" -and $_.NextHop -ne "::"} | remove-netroute -Confirm:$false

