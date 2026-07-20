package certificaterotation

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/plan"
)

const (
	windowsIdempotentScriptPath = `c:\var\lib\rancher\capr\idempotence\idempotent.ps1`
	windowsIdempotencyRoot      = `c:\var\lib\rancher\capr`
)

// Windows idempotent PowerShell action script (local copy of CAPR helper).
const windowsIdempotentActionScript = `param (
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

func windowsIdempotentActionScriptPath() string {
	return windowsIdempotentScriptPath
}

func windowsIdempotentScriptFile() plan.File {
	return plan.File{
		Content: base64.StdEncoding.EncodeToString([]byte(windowsIdempotentActionScript)),
		Path:    windowsIdempotentActionScriptPath(),
		Dynamic: true,
		Minor:   true,
	}
}

// windowsIdempotentInstruction creates a one-time instruction that runs the Windows idempotent action script.
func windowsIdempotentInstruction(identifier, value, command string, args []string, env []string) plan.OneTimeInstruction {
	hashedCommand := plan.PlanHash([]byte(command))
	hashedValue := plan.PlanHash([]byte(value))

	return plan.OneTimeInstruction{
		CommonInstruction: plan.CommonInstruction{
			Name:    fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand),
			Command: "powershell.exe",
			Args: append([]string{
				windowsIdempotentActionScriptPath(),
				strings.ToLower(identifier),
				hashedValue,
				hashedCommand,
				command,
				windowsIdempotencyRoot,
			}, args...),
			Env: env,
		},
	}
}

func windowsIdempotentRestartInstructions(identifier, value, service string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		windowsIdempotentInstruction(identifier+"-restart", value, "restart-service", []string{service}, []string{}),
	}
}
