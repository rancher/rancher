package planner

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
)

const (
	windowsIdempotentActionScript = `
param (
    [Parameter(Position=0)]
    [String]
    $Key,

    [Parameter(Position=1)]
    [String]
    $TargetHash,

    [Parameter(Position=2)]
    [String]
    $HashedCommand,

    [Parameter(Position=3)]
    [String]
    $Command,

    [Parameter(Position=4)]
    [String]
    $CAPRDir,

    [Parameter(Position=5, ValueFromRemainingArguments)]
    [String[]]
    $Args
)

$ErrorActionPreference = 'Stop'
$dataRoot = "$CAPRDir/idempotence/$Key/$HashedCommand/$TargetHash"
$attemptFile = "$dataRoot/last-attempt"
$currentAttempt = (Get-Content $attemptFile -ErrorAction Ignore)

if (($null -eq $currentAttempt) -or ($currentAttempt -eq "")) {
	$currentAttempt = "-1"
}

if ($currentAttempt -ne $env:CATTLE_AGENT_ATTEMPT_NUMBER) {
	if (-not (Test-Path $dataRoot)) {
		New-Item -Type Directory $dataRoot
	}

	Set-Content -Path $attemptFile -Value $env:CATTLE_AGENT_ATTEMPT_NUMBER

  	$joinedArgs = $Args -join ' '
	$fullCommand = ($Command + " '" + $joinedArgs + "'") 

	Invoke-Expression $fullCommand

} else { 
	Write-Host "action has already been reconciled to the target hash $TargetHash at attempt $currentAttempt"
}
`

	setPermissionsWindowsScriptPath = "%s/windows/set-permissions.ps1"
	setPermissionsWindowsScript     = `
function Set-RestrictedPermissions {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory=$true)]
        [string]
        $Path,
        [Parameter(Mandatory=$true)]
        [Boolean]
        $Directory
    )
    $Owner = "BUILTIN\Administrators"
    $Group = "NT AUTHORITY\SYSTEM"
    $acl = Get-Acl $Path
    
    foreach ($rule in $acl.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])) {
        $acl.RemoveAccessRule($rule) | Out-Null
    }
    $acl.SetAccessRuleProtection($true, $false)
    $acl.SetOwner((New-Object System.Security.Principal.NTAccount($Owner)))
    $acl.SetGroup((New-Object System.Security.Principal.NTAccount($Group)))
    
    Set-FileSystemAccessRule -Directory $Directory -acl $acl

    $FullPath = Resolve-Path $Path
    Write-Host "Setting restricted ACL on $FullPath"
    Set-Acl -Path $Path -AclObject $acl
}

function Set-FileSystemAccessRule() {
    [CmdletBinding()]
    param (
        [Parameter(Mandatory=$true)]
        [Boolean]
        $Directory,
        [Parameter(Mandatory=$false)]
        [System.Security.AccessControl.ObjectSecurity]
        $acl
    )
    $users = @(
        $acl.Owner,
        $acl.Group
    )
	# Note that the function signature for files and directories 
	# intentionally differ. 
    if ($Directory -eq $true) {
        foreach ($user in $users) {
            $rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
                $user,
                [System.Security.AccessControl.FileSystemRights]::FullControl,
                [System.Security.AccessControl.InheritanceFlags]'ObjectInherit,ContainerInherit',
                [System.Security.AccessControl.PropagationFlags]::None,
                [System.Security.AccessControl.AccessControlType]::Allow
            )
            $acl.AddAccessRule($rule)
        }
    } else {
        foreach ($user in $users) {
            $rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
                $user,
                [System.Security.AccessControl.FileSystemRights]::FullControl,
                [System.Security.AccessControl.AccessControlType]::Allow
            )
            $acl.AddAccessRule($rule)
        }
    }
}

function Confirm-ACL { 
	[CmdletBinding()]
	param (
		[Parameter(Mandatory=$true)]
		[String]
		$Path
	)
	foreach ($a in (Get-Acl $path).Access) {
		$ref = $a.IdentityReference
		if (($ref -ne "BUILTIN\Administrators") -and ($ref -ne "NT AUTHORITY\SYSTEM")) { 
			return $false
		}
	}
	return $true
}

$RKE2_DATA_DIR="%s"
$SYSTEM_AGENT_DIR="%s"
$RANCHER_PROVISIONING_DIR="%s"

$restrictedPaths = @(
	[PSCustomObject]@{
		Path = "c:\etc\rancher\wins\config"
		Directory = $false
	}
	[PSCustomObject]@{
		Path = "c:\etc\rancher\node\password"
		Directory = $false
	}
	[PSCustomObject]@{
		Path = "$SYSTEM_AGENT_DIR\rancher2_connection_info.json"
		Directory = $false
	}
	[PSCustomObject]@{
		Path = "c:\etc\rancher\rke2\config.yaml.d\50-rancher.yaml"
		Directory = $false
	}
	[PSCustomObject]@{
		Path = "c:\usr\local\bin\rke2.exe"
		Directory = $false
	}
	[PSCustomObject]@{
		Path = "$RANCHER_PROVISIONING_DIR"
		Directory = $true
	}
	[PSCustomObject]@{
		Path = "$SYSTEM_AGENT_DIR"
		Directory = $true
	}
	[PSCustomObject]@{
		Path = "$RKE2_DATA_DIR"
		Directory = $true
	}
)

foreach ($path in $restrictedPaths) {
	if (-Not (Confirm-ACL -Path $path.Path)) {
		Set-RestrictedPermissions -Path $path.Path -Directory $path.Directory
	}
}
`
)

var (
	setPermissionsWindowsScriptFile = plan.File{
		Content: base64.StdEncoding.EncodeToString(
			fmt.Appendf(nil, setPermissionsWindowsScript,
				"c:\\var\\lib\\rancher\\rke2",
				"c:\\var\\lib\\rancher\\agent",
				"c:\\var\\lib\\rancher\\capr")),

		Path: fmt.Sprintf(setPermissionsWindowsScriptPath,
			"c:\\var\\lib\\rancher\\capr"),
		Dynamic: true,
		Minor:   true,
	}
	setPermissionsWindowsScriptInstruction = plan.OneTimeInstruction{
		Name:    "Set permissions for RKE2 installation files on Windows",
		Command: "powershell.exe",
		Args: []string{"-File", fmt.Sprintf(setPermissionsWindowsScriptPath,
			"c:\\var\\lib\\rancher\\capr")},
	}
)

func windowsIdempotentActionScriptPath() string {
	// note: custom data directory paths are not currently respected by Windows nodes
	return "c:\\var\\lib\\rancher\\capr\\idempotence\\idempotent.ps1"
}

// windowsIdempotentRestartInstructions generates an idempotent restart instruction for the given Windows service.
// identifier is expected to be a unique key for tracking, and value should be something like the generation of the attempt
// (and is what we track to determine whether we should run the instruction or not).
func windowsIdempotentRestartInstructions(identifier, value, service string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		windowsIdempotentInstruction(
			identifier+"-restart",
			value,
			"restart-service",
			[]string{
				service,
			},
			[]string{},
		),
	}
}

// idempotentInstruction generates an idempotent action instruction that will execute the given command + args exactly once using PowerShell.
// It works by running a script that writes the given "value" to a file at /var/lib/rancher/capr/idempotence/<identifier>/<hashedCommand>/<targetHash>,
// and checks this file to determine if it needs to run the instruction again. Notably, `identifier` must be a valid relative path.
// The command provided must be a valid PowerShell expression which can be parsed by Invoke-Expression. Passing an executable as the command
// (such as powershell.exe) may result in error suppression. Due to how PowerShell handles arguments when executing commands via InvokeExpression,
// care must be taken to ensure that certain escape characters (such as ') do not interfere with how arguments are built and passed to InvokeExpression.
// Reference windowsIdempotentActionScript for more information as to how command arguments are crafted and passed to InvokeExpression.
func windowsIdempotentInstruction(identifier, value, command string, args []string, env []string) plan.OneTimeInstruction {
	hashedCommand := PlanHash([]byte(command))
	hashedValue := PlanHash([]byte(value))

	return plan.OneTimeInstruction{
		Name:    fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand),
		Command: "powershell.exe",
		Args: append([]string{
			windowsIdempotentActionScriptPath(),
			strings.ToLower(identifier),
			hashedValue,
			hashedCommand,
			command,
			// note: custom data directory paths are not currently respected by Windows nodes
			"c:\\var\\lib\\rancher\\capr",
		},
			args...),
		Env: env,
	}
}
