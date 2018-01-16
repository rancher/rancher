param(
    [Parameter(ValueFromPipeline=$true)]
    [string]$inputStr
)

$downloadFolder="download"
$rancherBaseDir="rancher"
$BasicLocation="c:\program files\$rancherBaseDir"
$AgentBinary="agent"
$PerHostSubnetBinary="per-host-subnet"
$rancherCompomentList="$PerHostSubnetBinary","$AgentBinary"
$strs=$inputStr.Split(",")
$HostLabels=$strs[1].Trim("`"")
$AgentIp=$strs[2].Trim("`"")
if($HostLabels -ne $null){
    [System.Environment]::SetEnvironmentVariable("CATTLE_HOST_LABELS",$HostLabels,"Machine")
}
if ($AgentIp -ne $null){
    [System.Environment]::SetEnvironmentVariable("CATTLE_AGENT_IP",$AgentIp,"Machine")
}
function Test-Updates {
    param(
        [string]$target=""
    )
    $rtn=$true
    for($i=0;$i -lt $rancherCompomentList.Count;$i++){ 
        $listTar=$rancherCompomentList[$i]
        if(("$target" -eq "$listTar") -or ("$target" -eq "")){
            $downloadHash=(Get-FileHash -Algorithm MD5 -Path "$pwd/$downloadFolder/$rancherBaseDir/$listTar.exe").Hash
            $downloadScriptHash=(Get-FileHash -Algorithm MD5 -Path "$pwd/$downloadFolder/$rancherBaseDir/startup_$listTar.ps1").Hash
            $currentHash=(Get-FileHash -Algorithm MD5 -Path "$pwd/$listTar.exe").Hash
            $currentScriptHash=(Get-FileHash -Algorithm MD5 -Path "$pwd/startup_$listTar.ps1").Hash
            if(("$downloadHash" -ne "$currentHash") -or ("$downloadScriptHash" -ne "$currentScriptHash")){
                $rtn = $rtn -and $true
            } else {
                $rtn = $false
            }
        }
    }
    return $rtn
}

function Update-Binaries  {
    param(
        [string]$target=""
    )
    for($i=0;$i -lt $rancherCompomentList.Count;$i++){ 
        $listTar=$rancherCompomentList[$i]
        if(("$target" -eq "$listTar") -or ("$target" -eq "")){
            Copy-Item -Path "$pwd/$downloadFolder/$rancherBaseDir/$listTar.exe" -Destination "$pwd/$listTar.exe"
            Copy-Item -Path "$pwd/$downloadFolder/$rancherBaseDir/startup_$listTar.ps1" -Destination "$pwd/startup_$listTar.ps1"
        }
    }
}

function Test-RancherComponent {
    param(
        [string]$target=""
    )
    $rtn=$true
    for($i=0;$i -lt $rancherCompomentList.Count;$i++){ 
        $listTar=$rancherCompomentList[$i]
        if(("$target" -eq "$listTar") -or ("$target" -eq "")){
            if($(Test-Path "$pwd/startup_$listTar.ps1") -and $(Test-Path "$pwd/$listTar.exe")){
                $rtn = $rtn -and $true
                
            } else {
                $rtn = $false
            }
        }
    }
    return $rtn
}

function Stop-RancherServices  {
    Stop-Service rancher-agent
    Stop-Service rancher-per-host-subnet
    Write-Host "Stop Services Success"
}

function Start-RancherServices  {
    Start-Service rancher-agent
    sleep 5
    Start-Service rancher-per-host-subnet
    Write-Host "Start Services Success"
}

try {
    Set-Location -Path $BasicLocation
    for($i=0;$i -lt $rancherCompomentList.Count;$i++){
        $listTar=$rancherCompomentList[$i]
        Write-Host "checking $listTar update"
        if(-not $(Test-RancherComponent "$listTar") -or $(Test-Updates "$listTar")){
            Update-Binaries "$listTar"
            Invoke-Expression "'$inputStr' | ./startup_$listTar.ps1"
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
