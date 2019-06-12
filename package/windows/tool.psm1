#########################################################################
# Log
#########################################################################
function Log-Info {
    Write-Host -NoNewline -ForegroundColor Blue "INFO "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))
}

function Log-Warn {
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))
}

function Log-Error {
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))
}

function Log-Fatal {
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($Args -join " "))

    exit 1
}

#########################################################################
# Output
#########################################################################
function Print {
    [System.Console]::Out.Write($Args -join " ")
    Start-Sleep -Milliseconds 100
}

function Error {
    [System.Console]::Error.Write($Args -join " ")
    Start-Sleep -Milliseconds 100
}

function Fatal {
    [System.Console]::Error.Write($Args -join " ")
    Start-Sleep -Milliseconds 100

    exit 1
}

#########################################################################
# Role
#########################################################################
function Is-Administrator {
    $p = New-Object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
    return $p.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
}

#########################################################################
# Environment
#########################################################################
function Set-Env {
    param(
        [parameter(Mandatory = $true)] [string]$Key,
        [parameter(Mandatory = $false)] [string]$Value = ""
    )

    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Process)
    [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Machine)
}

function Get-Env {
    param(
        [parameter(Mandatory = $true)] [string]$Key
    )

    $val = [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Process)
    if ($val) {
        return $val
    }

    return [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Machine)
}

#########################################################################
# HTTP
#########################################################################
function get-SkipServerCertificateValidation {
    $signature = @'
using System.Net;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;
public static class SkipServerCertificateValidation
{
    private static bool Callback(object sender, X509Certificate certificate, X509Chain chain, SslPolicyErrors sslPolicyErrors) { return true; }
    public static void Enable() {
        ServicePointManager.ServerCertificateValidationCallback = Callback;
        ServicePointManager.SecurityProtocol = SecurityProtocolType.Ssl3 | SecurityProtocolType.Tls | SecurityProtocolType.Tls11 | SecurityProtocolType.Tls12;
    }
    public static void Disable() {
        ServicePointManager.ServerCertificateValidationCallback = null;
        ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls | SecurityProtocolType.Tls11 | SecurityProtocolType.Tls12;
    }
}
'@
    if (-not ([System.Management.Automation.PSTypeName]"SkipServerCertificateValidation").Type) {
        Add-Type -TypeDefinition $signature
    }

    return [SkipServerCertificateValidation]
}

function Scrape-Content {
    param(
        [parameter(Mandatory = $true)]  [string]$Uri,
        [parameter(Mandatory = $false)] $Headers = @{"Cache-Control"="no-cache"},
        [parameter(Mandatory = $false)] [int32]$TimeoutSec = 15,
        [parameter(Mandatory = $false)] [switch]$SkipCertificateCheck
    )

    # in Powershell 6, we can set `-MaximumRetryCount 6 -RetryIntervalSec 10` to make this even more robust
    if ($PSVersionTable.PSVersion.Major -ge 6) {
        if ($SkipCertificateCheck) {
            return Invoke-RestMethod -Uri $Uri `
                    -Headers $Headers `
                    -TimeoutSec $TimeoutSec `
                    -MaximumRetryCount 6 `
                    -RetryIntervalSec 10 `
                    -SkipCertificateCheck
        }

        return Invoke-RestMethod -Uri $Uri `
                -Headers $Headers `
                -TimeoutSec $TimeoutSec `
                -MaximumRetryCount 6 `
                -RetryIntervalSec 10
    }

    if ($SkipCertificateCheck) {
        (get-SkipServerCertificateValidation)::Enable()
    }

    $ret = Invoke-RestMethod -Uri $Uri `
            -Headers $Headers `
            -TimeoutSec $TimeoutSec

    if ($SkipCertificateCheck) {
        (get-SkipServerCertificateValidation)::Disable()
    }

    return $ret
}

function validate-SHA1 {
    param(
        [parameter(Mandatory=$true)] [string]$Hash,
        [parameter(Mandatory=$true)] [string]$Path
    )

    $actualHash = (Get-FileHash -Path $Path -Algorithm SHA1).Hash

    if ($actualHash.ToLower() -ne $Hash.ToLower()) {
        throw ("$Path corrupted, sha1 $actualHash doesn't match expected $Hash")
    }
}

function Download-File {
    param(
        [parameter(Mandatory = $false)] [string]$Hash,
        [parameter(Mandatory = $true)]  [string]$OutFile,
        [parameter(Mandatory = $true)]  [string]$Uri,
        [parameter(Mandatory = $false)] $Headers = @{},
        [parameter(Mandatory = $false)] [int32]$TimeoutSec = 15,
        [parameter(Mandatory = $false)] [switch]$SkipCertificateCheck
    )

    # in Powershell 6, we can set `-MaximumRetryCount 6 -RetryIntervalSec 10` to make this even more robust
    if ($PSVersionTable.PSVersion.Major -ge 6) {
        if ($SkipCertificateCheck) {
            Invoke-WebRequest -UseBasicParsing `
                -TimeoutSec $TimeoutSec `
                -Uri $Uri `
                -OutFile $OutFile `
                -MaximumRetryCount 6 `
                -RetryIntervalSec 10 `
                -SkipCertificateCheck
        } else {
            Invoke-WebRequest -UseBasicParsing `
                -TimeoutSec $TimeoutSec `
                -Uri $Uri `
                -MaximumRetryCount 6 `
                -RetryIntervalSec 10 `
                -OutFile $OutFile
        }

        if ($Hash) {
            validate-SHA1 -Hash $Hash -Path $OutFile
        }

        return
    }

    if ($SkipCertificateCheck) {
        (get-SkipServerCertificateValidation)::Enable()
    }

    Invoke-WebRequest -UseBasicParsing `
        -TimeoutSec $TimeoutSec `
        -Uri $Uri `
        -OutFile $OutFile

    if ($SkipCertificateCheck) {
        (get-SkipServerCertificateValidation)::Disable()
    }

    if ($Hash) {
        validate-SHA1 -Hash $Hash -Path $OutFile
    }
}

#########################################################################
# Installation
#########################################################################
function Install-MSI {
    param(
        [parameter(Mandatory = $true)] [string]$File,
        [parameter(Mandatory = $true)] [string]$LogFile
    )

    $installArgs = @(
        "/i"
        $File
        "/qn"
        "/norestart"
        "/le"
        $LogFile
    )
    Start-Process -FilePath "msiexec.exe" -ArgumentList $installArgs -Wait -NoNewWindow
}

function Install-MSU {
    param(
        [parameter(Mandatory = $true)] [string]$File
    )

    $installArgs = @(
        $File
        "/quiet"
        "/norestart"
    )
    Start-Process -FilePath "wusa.exe" -ArgumentList $installArgs -Wait -NoNewWindow
}

#########################################################################
# Execution
#########################################################################
function Execute-Binary {
    param (
        [parameter(Mandatory = $true)] [string]$FilePath,
        [parameter(Mandatory = $false)] [string[]]$ArgumentList,
        [parameter(Mandatory = $false)] [switch]$PassThru
    )

    if (-not $PassThru) {
        if ($ArgumentList) {
            Start-Process -NoNewWindow -Wait `
                -FilePath $FilePath `
                -ArgumentList $ArgumentList
        } else {
            Start-Process -NoNewWindow -Wait `
                -FilePath $FilePath
        }
        return
    }

    $stdout = New-TemporaryFile
    $stderr = New-TemporaryFile
    $stdoutContent = ""
    $stderrContent = ""
    try {
        if ($ArgumentList) {
            Start-Process -NoNewWindow -Wait `
                -FilePath $FilePath `
                -ArgumentList $ArgumentList `
                -RedirectStandardOutput $stdout.FullName `
                -RedirectStandardError $stderr.FullName `
                -ErrorAction Ignore
        } else {
            Start-Process -NoNewWindow -Wait `
                -FilePath $FilePath `
                -RedirectStandardOutput $stdout.FullName `
                -RedirectStandardError $stderr.FullName `
                -ErrorAction Ignore
        }

        $stdoutContent = Get-Content $stdout.FullName
        $stderrContent = Get-Content $stderr.FullName
    } catch {
        $stderrContent = $_.Exception.Message
    }

    $stdout.Delete()
    $stderr.Delete()

    return @{} | Add-Member -NotePropertyMembers @{
        StdOut = $stdoutContent
        StdErr = $stderrContent
        Success = [string]::IsNullOrEmpty($stderrContent)
    } -PassThru
}


#########################################################################

Export-ModuleMember -Function Log-Info
Export-ModuleMember -Function Log-Warn
Export-ModuleMember -Function Log-Error
Export-ModuleMember -Function Log-Fatal

Export-ModuleMember -Function Print
Export-ModuleMember -Function Error
Export-ModuleMember -Function Fatal

Export-ModuleMember -Function Is-Administrator

Export-ModuleMember -Function Set-Env
Export-ModuleMember -Function Get-Env

Export-ModuleMember -Function Scrape-Content
Export-ModuleMember -Function Download-File

Export-ModuleMember -Function Install-MSI
Export-ModuleMember -Function Install-MSU

Export-ModuleMember -Function Execute-Binary
