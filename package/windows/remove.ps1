#Requires -Version 5.0

param (
)

$ErrorActionPreference = 'Stop'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'

#########################################################################
## START main definition

$rancherDir = "C:\etc\rancher"
$kubeDir = "C:\etc\kubernetes"
$cniDir = "C:\etc\cni"

## END main definition
#########################################################################
## START main execution

# 0 - success
# 1 - crash
# 2 - agent retry
trap {
    $errMsg = $_.Exception.Message

    popd

    [System.Console]::Error.Write($errMsg)

    if ($errMsg.EndsWith("agent retry")) {
        exit 2
    }

    exit 1
}


# check powershell #
if ($PSVersionTable.PSVersion.Major -lt 5) {
    throw "PowerShell version 5 or higher is required, crash"
}

# check identity #
$currentPrincipal = new-object System.Security.Principal.WindowsPrincipal([System.Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "You need elevated Administrator privileges in order to run this script, start Windows PowerShell by using the Run as Administrator option, crash"
}

# cniDir #
try {
    Remove-Item -Path $cniDir -Recurse -Force
    Remove-Item -Path "C:\etc\kube-flannel" -Recurse -Force -ErrorAction Ignore
} catch {
    throw ("Can't remove cniDir: {0}, crash" -f $cniDir)
}

# kubeDir #
try {
    Remove-Item -Path $kubeDir -Recurse -Force
} catch {
    throw ("Can't remove kubeDir: {0}, crash" -f $kubeDir)
}

## END main execution
#########################################################################