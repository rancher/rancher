<powershell>
# Start
Set-ExecutionPolicy -ExecutionPolicy Unrestricted -Scope LocalMachine -Force -ErrorAction Ignore

# Don't set this before Set-ExecutionPolicy as it throws an error
$ErrorActionPreference = "stop"
 
# Remove any existing Windows Management listeners
Remove-Item -Path WSMan:\Localhost\listener\listener* -Recurse

# Create self-signed cert for encrypted WinRM on port 5986
$Cert = New-SelfSignedCertificate -CertstoreLocation Cert:\LocalMachine\My -DnsName $ENV:COMPUTERNAME
New-Item -Path WSMan:\LocalHost\Listener -Transport HTTPS -Address * -CertificateThumbPrint $Cert.Thumbprint -Force

# Configure UAC to allow privilege elevation in remote shells
$Key = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System'
$Setting = 'LocalAccountTokenFilterPolicy'
Set-ItemProperty -Path $Key -Name $Setting -Value 1 -Force

# Disable Real time monitoring
cmd.exe /c netsh advfirewall set allprofiles state off
 
# Configure WinRM to allow encrypted communication, and provide the self-signed cert to the WinRM listener
cmd.exe /c winrm quickconfig -q
cmd.exe /c winrm set "winrm/config" '@{MaxTimeoutms="1800000"}'
cmd.exe /c winrm set "winrm/config/winrs" '@{MaxMemoryPerShellMB="1024"}'
cmd.exe /c winrm set "winrm/config/service" '@{AllowUnencrypted="false"}'
cmd.exe /c winrm set "winrm/config/client" '@{AllowUnencrypted="false"}'
cmd.exe /c winrm set "winrm/config/service/auth" '@{Basic="true"}'
cmd.exe /c winrm set "winrm/config/client/auth" '@{Basic="true"}'
cmd.exe /c winrm set "winrm/config/service/auth" '@{CredSSP="true"}'
cmd.exe /c winrm set "winrm/config/listener?Address=*+Transport=HTTPS" "@{Port=`"5986`";Hostname=`"$($ENV:COMPUTERNAME)`";CertificateThumbprint=`"$($Cert.Thumbprint)`"}"
cmd.exe /c netsh advfirewall firewall add rule profile=any name="Allow WinRM HTTPS" dir=in localport=5986 protocol=TCP action=allow

# Restart WinRM, and set it so that it auto-launches on startup
Stop-Service -Name "WinRM" -ErrorAction Stop
Set-Service -Name "WinRM" -StartupType Automatic
Start-Service -Name "WinRM" -ErrorAction Stop
</powershell>