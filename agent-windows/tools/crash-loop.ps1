Param(
    [string]$target
)

Import-Module -Name "c:\program files\rancher\rancher-tools.psm1" -Verbose
Add-LogContent "$target" "start crash loop"
$rancherBaseDir="rancher"
$downloadFolder="download"
$BasicLocation="c:\program files\$rancherBaseDir"
$lockFileName="$BasicLocation\$target-lock"
$HostLabels=[System.Environment]::GetEnvironmentVariable("CATTLE_HOST_LABELS","Machine")
$AgentIp=[System.Environment]::GetEnvironmentVariable("CATTLE_AGENT_IP","Machine")
$RegisterUrl=[System.Environment]::GetEnvironmentVariable("CATTLE_REGISTER_URL","Machine")
$inputStr="`"$RegisterUrl`",`"$HostLabels`",`"$AgentIp`""

Set-Location $BasicLocation
if(Test-Path "$lockFileName"){
    exit 0
}
Start-Sleep 5

try{
    $key,$secret,$url,$this_ip=$(Register-Node $RegisterUrl)
    Set-Content $lockFileName -Value "locking" -ErrorVariable $err
    if($err -ne $null){
        exit 0
    }
    $downloadlist=Get-DownloadListByTarget -target $target
    foreach($item in $downloadList.Keys){
        $zipFile=$downloadList[$item]
        Get-RancherBinary $key $secret $url $item $zipFile
        Expand-Archive "$zipFile" -DestinationPath "./$downloadFolder" -Force
        Remove-Item -Path "$zipFile"
        if(-not $(Test-RancherComponent "$target")){
            Update-Binaries "$target"
            Add-LogContent "$target" "$($(Get-Date).ToString()): Update $target success"
        }
    }
}
catch {
    $err=$Error[0].ToString()
    Add-LogContent "$target" "$($(Get-Date).ToString()): Updating $target after crash fail because $err"
}
finally{
    Add-LogContent "$target" "prepare to start $target"
    Start-RancherCompoment $target $inputStr
}

Remove-Item $lockFileName