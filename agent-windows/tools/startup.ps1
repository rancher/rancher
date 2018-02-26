param(
    [Parameter(ValueFromPipeline=$true)]
    [string]$inputStr
)
$downloadFolder="download"
$rancherBaseDir="rancher"
$BasicLocation="c:\program files\$rancherBaseDir"
Import-Module -Name "c:\program files\$rancherBaseDir\rancher-tools.psm1" -Verbose

$rancherComponentList=Get-RancherComponentList
$strs=$inputStr.Split(",")
$RegisterUrl=$strs[0].Trim("`"")
$HostLabels=$strs[1].Trim("`"")
$AgentIp=$strs[2].Trim("`"")

if(($HostLabels -ne $null) -and ($HostLabels -ne "")){
    [System.Environment]::SetEnvironmentVariable("CATTLE_HOST_LABELS",$HostLabels,"Machine")
}
if (($AgentIp -ne $null) -and ($AgentIp -ne "")){
    [System.Environment]::SetEnvironmentVariable("CATTLE_AGENT_IP",$AgentIp,"Machine")
}
if ($RegisterUrl -ne ""){
    [System.Environment]::SetEnvironmentVariable("CATTLE_REGISTER_URL",$RegisterUrl,"Machine")
}

try {
    Set-Location -Path $BasicLocation
    $key,$secret,$url,$this_ip=$(Register-Node $RegisterUrl)
    if ($AgentIp -ne $null) { $this_ip=$AgentIp}
    if (Test-Path "$downloadFolder"){Remove-Item -Path "$downloadFolder" -Confirm:$false -Recurse -Force }
    New-Item -ItemType Directory -Path "$downloadFolder" > $null
    
    foreach($compoment in $rancherComponentList){
        $downloadList=Get-DownloadListByTarget $compoment
        foreach($target in $downloadList.Keys){
            $zipFile=$downloadList[$target]
            Get-RancherBinary $key $secret $url $target $zipFile
            Expand-Archive "$zipFile" -DestinationPath "./$downloadFolder"
            Remove-Item -Path "$zipFile"
        }
    }

    for($i=0;$i -lt $rancherComponentList.Count;$i++){
        $listTar=$rancherComponentList[$i]
        Write-Host "checking $listTar update"
        if(-not $(Test-RancherComponent "$listTar")){
            Stop-RancherComponent "$listTar"
            Update-Binaries "$listTar"
            Start-RancherComponent $listTar $inputStr
            Write-Host "create or update $listTar success"
        } else{
            Write-Host "version of $listTar is the same, do nothing"
        }
    }  
}
catch {
    Write-Host $Error[0]
    Write-Host "Get error while running startup.ps1"
}
