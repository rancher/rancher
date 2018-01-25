[Reflection.Assembly]::LoadWithPartialName("System.Web") | Out-Null

function Get-RancherLabel {
    Param(
        # Parameter help description
        [string]$LabelKey
    )
    $RancherLabelKey="CATTLE_HOST_LABELS"
    try{
        $labels= [System.Environment]::GetEnvironmentVariable("$RancherLabelKey","Machine")
        if($labels -eq ""){
            return ""
        }
        $qs=[System.Web.HttpUtility]::ParseQueryString($LabelKey)
        return $qs["$Key"]
    }
    catch{
     return ""
    }
}

function Test-RancherNetwork {
    param(
        [string]$Subnet
    )
    $defaultNetworkName = "transparent"
    $output=docker network inspect $defaultNetworkName 2>$null
    if ("$output" -eq "[]"){
        return $false
    }
    $Network= ConvertFrom-Json "$output"
    if("$Subnet" -ne $network.IPAM.Config.subnet){
        return $false
    }
    return $true
}

Export-ModuleMember -function Get-RancherLabel
Export-ModuleMember -Function Test-RancherNetwork

