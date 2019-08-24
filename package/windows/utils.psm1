$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

function Log-Info
{
    Write-Host -NoNewline -ForegroundColor Blue "INFO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Warn
{
    Write-Host -NoNewline -ForegroundColor DarkYellow "WARN: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Error
{
    Write-Host -NoNewline -ForegroundColor DarkRed "ERRO: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))
}

function Log-Fatal
{
    Write-Host -NoNewline -ForegroundColor DarkRed "FATA: "
    Write-Host -ForegroundColor Gray ("{0,-44}" -f ($args -join " "))

    exit 255
}

function Is-Administrator
{
    $p = New-Object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
    return $p.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
}

function ConvertTo-JsonObj
{
    param (
        [parameter(Mandatory = $false, ValueFromPipeline = $true)] [string]$JSON
    )

    if (-not $JSON) {
        return $null
    }

    try {
        $ret = $JSON | ConvertFrom-Json -ErrorAction Ignore -WarningAction Ignore
        return $ret
    } catch {
        return $null
    }
}

function Set-Env
{
    param(
        [parameter(Mandatory = $true)] [string]$Key,
        [parameter(Mandatory = $false)] [string]$Value = ""
    )

    try {
        [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Process)
    } catch {
        Log-Error "Could not set $Key = $Value in Process target: $($_.Exception.Message)"
    }

    try {
        [Environment]::SetEnvironmentVariable($Key, $Value, [EnvironmentVariableTarget]::Machine)
    } catch {
        Log-Error "Could not set $Key = $Value in Machine target: $($_.Exception.Message)"
    }
}

function Get-Env
{
    param(
        [parameter(Mandatory = $true)] [string]$Key
    )

    try {
        $val = [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Process)
        if ($val) {
            return $val
        }
    } catch {
        Log-Error "Could not get $Key in Process target: $($_.Exception.Message)"
    }

    try {
        $val = [Environment]::GetEnvironmentVariable($Key, [EnvironmentVariableTarget]::Machine)
        if ($val) {
            return $val
        }
    } catch {
        Log-Error "Could not get $Key in Machine target: $($_.Exception.Message)"
    }

    return ""
}

function Get-VmComputeNativeMethods()
{
    $ret = 'VmCompute.PrivatePInvoke.NativeMethods' -as [type]
    if (-not $ret) {
        $signature = @'
[DllImport("vmcompute.dll")]
public static extern void HNSCall([MarshalAs(UnmanagedType.LPWStr)] string method, [MarshalAs(UnmanagedType.LPWStr)] string path, [MarshalAs(UnmanagedType.LPWStr)] string request, [MarshalAs(UnmanagedType.LPWStr)] out string response);
'@
        $ret = Add-Type -MemberDefinition $signature -Namespace VmCompute.PrivatePInvoke -Name "NativeMethods" -PassThru
    }
    return $ret
}

function Invoke-HNSRequest
{
    param
    (
        [ValidateSet('GET', 'POST', 'DELETE')]
        [parameter(Mandatory = $true)] [string] $Method,
        [ValidateSet('networks', 'endpoints', 'activities', 'policylists', 'endpointstats', 'plugins')]
        [parameter(Mandatory = $true)] [string] $Type,
        [parameter(Mandatory = $false)] [string] $Action,
        [parameter(Mandatory = $false)] [string] $Data = "",
        [parameter(Mandatory = $false)] [Guid] $Id = [Guid]::Empty
    )

    $hnsPath = "/$Type"
    if ($id -ne [Guid]::Empty) {
        $hnsPath += "/$id"
    }
    if ($Action) {
        $hnsPath += "/$Action"
    }

    $response = ""
    $hnsApi = Get-VmComputeNativeMethods
    $hnsApi::HNSCall($Method, $hnsPath, "$Data", [ref]$response)

    $output = @()
    if ($response) {
        try {
            $output = ($response | ConvertFrom-Json)
            if ($output.Error) {
                Log-Error $output;
            } else {
                $output = $output.Output;
            }
        } catch {
            Log-Error $_.Exception.Message
        }
    }

    return $output;
}

Export-ModuleMember -Function Log-Info
Export-ModuleMember -Function Log-Warn
Export-ModuleMember -Function Log-Error
Export-ModuleMember -Function Log-Fatal
Export-ModuleMember -Function Is-Administrator
Export-ModuleMember -Function ConvertTo-JsonObj
Export-ModuleMember -Function Set-Env
Export-ModuleMember -Function Get-Env
Export-ModuleMember -Function Get-VmComputeNativeMethods
Export-ModuleMember -Function Invoke-HNSRequest

