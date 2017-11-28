param(
    [string]$RegisterUrl=$(throw "Error missing: -RegisterUrl <url>"),
    [string]$HostLabels,
    [string]$AgentIp
)

function register {
    param(
        [string]$RegisterUrl
    )
    process {
        $resp=$(Invoke-WebRequest -Method Get -Uri "$RegisterUrl")
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
        $auth=getBasicAuth $CATTLE_REGISTRATION_ACCESS_KEY $CATTLE_REGISTRATION_SECRET_KEY
        $resp=$(Invoke-WebRequest -Method POST -Headers @{"Authorization"="$auth";"Content-Type"="application/json"} -Body "$json" -Uri $CATTLE_URL/register  -UserAgent "curl" -ErrorVariable $err)
        if($err -ne $null){
            return 
        }
        $trueKey,$trueSecret=$(getTrueKeys $token)
        return $trueKey,$trueSecret,$CATTLE_URL,$DETECTED_CATTLE_AGENT_IP
    }
}

function getTrueKeys {
    param(
        [string]$token
    )
    process{
        $filters="key=$token"
        for ($i = 0; $i -lt 5; $i++) {
            $resp=$(Invoke-WebRequest -Method Get -Headers @{"Authorization"="$auth"} -Uri "$CATTLE_URL/register?$filters" -UserAgent "curl" -ErrorVariable $err)
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

function getBasicAuth{
    param(
        [string]$CATTLE_REGISTRATION_ACCESS_KEY,
        [string]$CATTLE_REGISTRATION_SECRET_KEY
    )
    process {
        return "Basic "+[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($CATTLE_REGISTRATION_ACCESS_KEY+":"+$CATTLE_REGISTRATION_SECRET_KEY))
    }
}

function getToken { 
    $output=""
    for ($i = 0; $i -lt 16; $i++) {
        $output+=$(-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 8 | ForEach-Object {[char]$_}))
    }
    return $output
}

function getBinary{
    param(
        [string]$key,
        [string]$secret,
        [string]$cattleUrl
    )
    process{
        $baseUri="/configContent/"
        $auth=getBasicAuth $key $secret
        foreach($objectName in $downloadNames.Keys){
            $filename="$($downloadNames[$objectName])"
            $tmpfilename= "$filename-tmp"
            $versionfilename="$filename-version"
            Invoke-WebRequest -Uri "$cattleUrl$baseUri$objectName" -Headers @{"Authorization"="$auth"} -UserAgent "curl" -OutFile "./$tmpfilename"
            $bytes=[system.io.file]::ReadAllBytes($tmpfilename)
            [system.io.file]::writeallbytes("$pwd/$versionfilename",$bytes[0..75])
            [system.io.file]::writeallbytes("$pwd/$filename",$bytes[75..$bytes.length])
            rm $tmpfilename
        }
    }
}

$downloadNames=@{"windows-agent"="go-agent.zip";"per-host-subnet"="rancher-per-host-subnet.zip"}
$rancherBaseDir="rancher"
$key,$secret,$url,$this_ip=$(register $RegisterUrl)
if ($AgentIp -ne $null) { $this_ip=$AgentIp}
getBinary $key $secret $url

foreach($zipFile in $downloadNames.Values){
    Expand-Archive "$zipFile" -DestinationPath .
}
Copy-item -Path "c:/program files/devcon.exe" -Destination "c:/program files/$rancherBaseDir/devcon.exe"
Write-Output "`"$RegisterUrl`",`"$HostLabels`",`"$AgentIp`""