[Reflection.Assembly]::LoadWithPartialName("System.Web") | Out-Null

$AgentBinary="agent"
$PerHostSubnetBinary="per-host-subnet"
$rancherComponentList="$PerHostSubnetBinary","$AgentBinary"
$startupPrefix="startup_"
$stopPrefix="stop_"
$servicePrefix="rancher-"
$downloadFolder="download"
$rancherBaseDir="rancher"
$BasicLocation="c:\program files\$rancherBaseDir"

function Update-Binaries  {
    param(
        [string]$target=""
    )
    for($i=0;$i -lt $rancherComponentList.Count;$i++){ 
        $listTar=$rancherComponentList[$i]
        if(("$target" -eq "$listTar") -or ("$target" -eq "")){
            Copy-Item -Path "$BasicLocation/$downloadFolder/$rancherBaseDir/$listTar.exe" -Destination "$BasicLocation/$listTar.exe"
            Copy-Item -Path "$BasicLocation/$downloadFolder/$rancherBaseDir/$startupPrefix$listTar.ps1" -Destination "$BasicLocation/$startupPrefix$listTar.ps1"
        }
    }
}

function Test-RancherComponent {
    param(
        [string]$target=""
    )
    $rtn=$true
    for($i=0;$i -lt $rancherComponentList.Count;$i++){ 
        $listTar=$rancherComponentList[$i]
        if(("$target" -eq "$listTar") -or ("$target" -eq "")){
            if($(Test-Path "$BasicLocation/$startupPrefix$listTar.ps1") -and $(Test-Path "$BasicLocation/$listTar.exe")){
                $rtn = $rtn -and $true
            } else {
                $rtn = $false
                continue
            }
            $downloadHash=(Get-FileHash -Algorithm MD5 -Path "$BasicLocation/$downloadFolder/$rancherBaseDir/$listTar.exe").Hash
            $downloadScriptHash=(Get-FileHash -Algorithm MD5 -Path "$BasicLocation/$downloadFolder/$rancherBaseDir/$startupPrefix$listTar.ps1").Hash
            $currentHash=(Get-FileHash -Algorithm MD5 -Path "$BasicLocation/$listTar.exe").Hash
            $currentScriptHash=(Get-FileHash -Algorithm MD5 -Path "$BasicLocation/$startupPrefix$listTar.ps1").Hash
            if(("$downloadHash" -eq "$currentHash") -and ("$downloadScriptHash" -eq "$currentScriptHash")){
                $rtn = $rtn -and $true
            } else {
                $rtn = $false
            }
        }
    }
    return $rtn
}

function getTrueKeys {
    param(
        [string]$token
    )
    process{
        $filters="key=$token"
        for ($i = 0; $i -lt 5; $i++) {
            $resp=$(Invoke-WebRequest -UseBasicParsing -Method Get -Headers @{"Authorization"="$auth"} -Uri "$CATTLE_URL/register?$filters" -UserAgent "curl" -ErrorVariable $err)
            if($err -ne $null){
                throw "get keys error" 
            }
            $respObj=ConvertFrom-Json $resp.Content
            if(($respObj.data -eq $null) -or ($respObj.data.length -eq 0) ){
                throw "can not get register with token"
            }
            $registerObject=$respObj.data[0]
            if($registerObject.state -eq "active"){
                return $registerObject.accessKey,$registerObject.secretKey
            }
            Start-Sleep 1
        }
        throw "get keys error, retry 5 times"
    }
}

function getToken { 
    $output= Get-Content C:\ProgramData\rancher\registrationToken -ErrorAction Ignore
    if($output -eq $null){
        $output=""
    } elseif($output -ne ""){
        return $output
    }
    for ($i = 0; $i -lt 16; $i++) {
        $output+=$(-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 8 | ForEach-Object {[char]$_}))
    }
    mkdir C:\ProgramData\rancher
    set-content C:\ProgramData\rancher\registrationToken -Value $output
    return $output
}
function Get-RancherLabel {
    Param(
        # Parameter help description
        [string]$Key
    )
    $RancherLabelKey="CATTLE_HOST_LABELS"
    try{
        $labels= [System.Environment]::GetEnvironmentVariable("$RancherLabelKey","Machine")
        if($labels -eq ""){
            return ""
        }
        $qs=[System.Web.HttpUtility]::ParseQueryString($labels)
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

function Get-RancherBasicAuth{
    param(
        [string]$CATTLE_REGISTRATION_ACCESS_KEY,
        [string]$CATTLE_REGISTRATION_SECRET_KEY
    )
    process {
        return "Basic "+[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($CATTLE_REGISTRATION_ACCESS_KEY+":"+$CATTLE_REGISTRATION_SECRET_KEY))
    }
}

function Register-Node {
    param(
        [string]$RegisterUrl
    )
    process {
        $resp=$(Invoke-WebRequest -UseBasicParsing -Method Get -Uri "$RegisterUrl")
        foreach($line in $resp.Content.Split("`n")){            
            if($line.Contains("CATTLE_REGISTRATION_ACCESS_KEY")){
                $CATTLE_REGISTRATION_ACCESS_KEY=$line.Split("=")[1].Trim('"')
            }
            elseif($line.Contains("CATTLE_REGISTRATION_SECRET_KEY")){
                $CATTLE_REGISTRATION_SECRET_KEY=$line.Split("=")[1].Trim('"')
            }
            elseif ($line.Contains("CATTLE_URL")) {
                $CATTLE_URL=$line.Split("=")[1].Trim('"')
            }elseif ($line.Contains("DETECTED_CATTLE_AGENT_IP")) {
                $DETECTED_CATTLE_AGENT_IP = $line.Split("=")[1].Trim('"')
            }
        }
        $token=$(getToken)
        $json= @{"key"="$token"} | ConvertTo-Json
        $auth=Get-RancherBasicAuth $CATTLE_REGISTRATION_ACCESS_KEY $CATTLE_REGISTRATION_SECRET_KEY
        $resp=$(Invoke-WebRequest -UseBasicParsing -Method POST -Headers @{"Authorization"="$auth";"Content-Type"="application/json"} -Body "$json" -Uri $CATTLE_URL/register  -UserAgent "curl" -ErrorVariable $err)
        if($err -ne $null){
            return 
        }
        $trueKey,$trueSecret=$(getTrueKeys $token)
        return $trueKey,$trueSecret,$CATTLE_URL,$DETECTED_CATTLE_AGENT_IP
    }
}

function Get-RancherBinary{
    param(
        [string]$key,
        [string]$secret,
        [string]$cattleUrl,
        [string]$objectName,
        [string]$fileName
    )
    process{
        $baseUri="/configContent/"
        $auth=Get-RancherBasicAuth $key $secret
        $tmpfilename= "$filename-tmp"
        $versionfilename="$filename-version"
        Invoke-WebRequest -UseBasicParsing -Uri "$cattleUrl$baseUri$objectName" -Headers @{"Authorization"="$auth"} -UserAgent "curl" -OutFile "$BasicLocation/$tmpfilename"
        $bytes=[system.io.file]::ReadAllBytes("$BasicLocation/$tmpfilename")
        #[system.io.file]::writeallbytes("$BasicLocation/$versionfilename",$bytes[0..75])
        [system.io.file]::writeallbytes("$BasicLocation/$filename",$bytes[75..$bytes.length])
        rm $tmpfilename
    }
}

function Get-DownloadList {
    return @{"windows-agent"="go-agent.zip";"per-host-subnet"="rancher-per-host-subnet.zip"}
}

function Get-DownloadListByTarget{
    Param(
        [string]$target
    )
    switch ($target) {
        "agent" {  
            return @{"windows-agent"="go-agent.zip"}
        }
        "per-host-subnet" {
            return @{"per-host-subnet"="rancher-per-host-subnet.zip"}
        }
    }
}

function Get-RancherComponentList {
    return $rancherComponentList
}

function Start-RancherComponent  {
    param(
        [string]$tar,
        [string]$inputStr
    )
    try{
        $script=$startupPrefix+$tar+".ps1"
        Invoke-Expression "./$script '$inputStr'" -ErrorVariable $err
        if($err -ne $null){
            throw $err
        }
        Write-Host "create or update $tar success"
    }
    catch{
        if($Error[0].ToString() -eq "ScriptHalted"){
            return
        }
        throw $Error[0]
    } 
}

function Stop-RancherComponent {
    param(
        [string]$tar
    )
    $serviceName="$servicePrefix"+"$tar"
    $stopScript="$BasicLocation"+"\"+"$stopPrefix"+"$tar"+".ps1"
    if(Test-Path "$stopScript"){
        Invoke-Expression "$stopScript"
    }else{
        $tarService=get-service "$servicename" -ErrorAction Ignore
        if($tarService -ne $null){
            $tarService | Set-Service -StartupType Manual -ErrorAction Ignore
            Stop-Service "$servicename" -ErrorAction Ignore
        }
    }
}

function Add-LogContent  {
    param(
        [string]$target,
        [string]$content
    )
    addLog "$BasicLocation\crash-loop-$target.log" "[Info]:$content"
}

function Add-DebugContent  {
    param(
        [string]$target,
        [string]$content
    )
    $DEBUG=[System.Environment]::GetEnvironmentVariable("CATTLE_HOST_DEBUG")
    if("$DEBUG" -eq "" ){
        return
    }
    addLog "$BasicLocation\crash-loop-$target.log" "[DEBUG]:$content"
}

function addLog{
    param(
        [string]$filename,
        [string]$content
    )
    "$content" >> "$filename"
}

Export-ModuleMember -function Get-RancherLabel
Export-ModuleMember -Function Test-RancherNetwork
Export-ModuleMember -Function Get-RancherBasicAuth
Export-ModuleMember -Function Update-Binaries
Export-ModuleMember -Function Test-RancherComponent
Export-ModuleMember -Function Get-DownloadList
Export-ModuleMember -Function Get-RancherComponentList
Export-ModuleMember -Function Start-RancherComponent 
Export-ModuleMember -Function Stop-RancherComponent 
Export-ModuleMember -Function Add-DebugContent
Export-ModuleMember -Function Add-LogContent
Export-ModuleMember -Function Register-Node
Export-ModuleMember -Function Get-DownloadListByTarget
Export-ModuleMember -Function Get-RancherBinary