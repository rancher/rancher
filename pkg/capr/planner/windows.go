package planner

import (
	"encoding/base64"
	"fmt"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
)

const (
	setPermissionsWindowsScriptPath = "%s/windows/set-permissions.ps1"

	setPermissionsWindowsScript = `
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
		Content: base64.StdEncoding.EncodeToString([]byte(
			fmt.Sprintf(setPermissionsWindowsScript,
				"c:\\var\\lib\\rancher\\rke2",
				"c:\\var\\lib\\rancher\\agent",
				"c:\\var\\lib\\rancher\\capr"))),

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
