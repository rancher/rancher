$IMAGE=$(get-item env:IMAGE -ErrorAction Ignore)

Invoke-WebRequest -Uri "https://github.com/rancher/windows-binaries/releases/download/v0.0.1/devcon.exe" -OutFile "tools/devcon.exe"
$byteArray=(Invoke-WebRequest -Uri "https://github.com/rancher/windows-binaries/releases/download/v0.0.1/MD5SUM").Content
$tarMD5=[System.Text.Encoding]::ASCII.GetString($byteArray).ToUpper()
$fileHash=(Get-FileHash -Path "tools/devcon.exe" -Algorithm MD5).Hash
if("$fileHash" -ne "$tarMD5"){
    Write-Host "MD5SUM of devcon.exe mismatch between local and remote storage."
    Exit 1
}

if($IMAGE -eq $null){
    $IMAGE=(Get-Content ./Dockerfile | Select-String RANCHER_AGENT_WINDOWS_IMAGE).line.split(" ")[2]
}

write-host "Building $IMAGE"
docker build -t ${IMAGE} .

remove-item -Path "$pwd/tools/devcon.exe"