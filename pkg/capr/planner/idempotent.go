package planner

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
)

const idempotentActionScript = `
#!/bin/sh

currentHash=""
key=$1
targetHash=$2
hashedCmd=$3
cmd=$4
shift 4

dataRoot="/var/lib/rancher/capr/idempotence/$key/$hashedCmd/$targetHash"
attemptFile="$dataRoot/last-attempt"

currentAttempt=$(cat "$attemptFile" || echo "-1")

if [ "$currentAttempt" != "$CATTLE_AGENT_ATTEMPT_NUMBER" ]; then
	mkdir -p "$dataRoot"
	echo "$CATTLE_AGENT_ATTEMPT_NUMBER" > "$attemptFile"
	exec "$cmd" "$@"
else
	echo "action has already been reconciled to the target hash $targetHash at attempt $currentAttempt"
fi
`

const (
	idempotentActionScriptPath = "/var/lib/rancher/capr/idempotence/idempotent.sh"
)

var idempotentScriptFile = plan.File{
	Content: base64.StdEncoding.EncodeToString([]byte(idempotentActionScript)),
	Path:    idempotentActionScriptPath,
	Dynamic: true,
	Minor:   true,
}

// idempotentInstruction generates an idempotent action instruction that will execute the given command + args exactly once.
// It works by running a script that writes the given "value" to a file at /var/lib/rancher/idempotence/<identifier>/<hashedCommand>,
// and checks this file to determine if it needs to run the instruction again. Notably, `identifier` must be a valid relative path.
func idempotentInstruction(identifier, value, command string, args []string, env []string) plan.OneTimeInstruction {
	hashedCommand := PlanHash([]byte(command))
	hashedValue := PlanHash([]byte(value))
	return plan.OneTimeInstruction{
		Name:    fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand),
		Command: "/bin/sh",
		Args: append([]string{
			"-x",
			idempotentActionScriptPath,
			strings.ToLower(identifier),
			hashedValue,
			hashedCommand,
			command},
			args...),
		Env: env,
	}
}

// convertToIdempotentInstruction converts a OneTimeInstruction to a OneTimeInstruction wrapped with the idempotent script.
// This is useful when an instruction may be used in various phases, without needing idempotency in all cases.
func convertToIdempotentInstruction(identifier, value string, instruction plan.OneTimeInstruction) plan.OneTimeInstruction {
	newInstruction := idempotentInstruction(identifier, value, instruction.Command, instruction.Args, instruction.Env)
	newInstruction.Image = instruction.Image
	newInstruction.SaveOutput = instruction.SaveOutput
	return newInstruction
}

// idempotentRestartInstructions generates an idempotent restart instructions for the given runtimeUnit. It checks the
// unit for failure, resets it if necessary, and restarts the unit.
func idempotentRestartInstructions(identifier, value, runtimeUnit string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		idempotentInstruction(
			identifier+"-reset-failed",
			value,
			"/bin/sh",
			[]string{
				"-c",
				fmt.Sprintf("if [ $(systemctl is-failed %s) = failed ]; then systemctl reset-failed %s; fi", runtimeUnit, runtimeUnit),
			},
			[]string{},
		),
		idempotentInstruction(
			identifier+"-restart",
			value,
			"systemctl",
			[]string{
				"restart",
				runtimeUnit,
			},
			[]string{},
		),
	}
}
