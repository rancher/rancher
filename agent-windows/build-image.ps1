$IMAGE=$(get-item env:IMAGE -ErrorAction Ignore)

Invoke-WebRequest -Uri "https://github.com/rancher/windows-binaries/releases/download/v0.0.1/devcon.exe" -OutFile devcon.exe
Invoke-WebRequest -Uri "https://github.com/rancher/windows-binaries/releases/download/v0.0.1/MD5SUM" -OutFile "MD5SUM"

$md5 = New-Object -TypeName System.Security.Cryptography.MD5CryptoServiceProvider
$hashBuilder = New-Object System.Text.StringBuilder
$md5.ComputeHash([System.IO.File]::ReadAllBytes($pwd.Path+"/devcon.exe")) | ForEach-Object { [void]$hashBuilder.Append($_.ToString("x2")) }
$targetHash = Get-Content -Path "$($pwd.Path)/MD5SUM"
if($hashBuilder.ToString() -ne "$targetHash"){
    Write-Host "MD5SUM of devcon.exe mismatch between local and remote storage."
    Exit 1
}

if($IMAGE -eq $null){
    $IMAGE=(Get-Content ./Dockerfile | Select-String RANCHER_AGENT_WINDOWS_IMAGE).line.split(" ")[2]
}

write-host "Building $IMAGE"
docker build -t ${IMAGE} .

remove-item -Path "$($pwd.Path)/devcon.exe"
remove-item -Path "$($pwd.Path)/MD5SUM"