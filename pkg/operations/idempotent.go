package operations

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	planapi "github.com/rancher/rancher/pkg/plan"
)

// IdempotentActionScript wraps a provided command in additional checks which ensure the command
// will be attempted until it is successfully run once, and then prevent it from being
// run again - effectively making arbitrary commands idempotent. This prevents
// potential re-execution of commands which must only be run once (etcd operations, etc.)
// but are not idempotent by default. The command will be reattempted until either it is
// successful, or the max-failures limit set for the plan is reached. The definition of
// $CATTLE_AGENT_ATTEMPT_NUMBER can be found in the system-agent repository, but it is effectively
// just the plans failure-count + 1.
const IdempotentActionScript = `
#!/bin/sh

currentHash=""
key=$1
targetHash=$2
hashedCmd=$3
cmd=$4
caprDir=$5
shift 5

dataRoot="$caprDir/idempotence/$key/$hashedCmd/$targetHash"
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

// IdempotentActionScriptPath returns the absolute path to the idempotent action script for the given provisioningDir.
func IdempotentActionScriptPath(provisioningDir string) string {
	return path.Join(provisioningDir, "idempotence/idempotent.sh")
}

// IdempotentScriptFile returns a planapi.File that installs the idempotent action script at provisioningDir.
// Plans that use IdempotentInstruction must include this file (or otherwise ensure the script is present).
func IdempotentScriptFile(provisioningDir string) planapi.File {
	return planapi.File{
		Content: base64.StdEncoding.EncodeToString([]byte(IdempotentActionScript)),
		Path:    IdempotentActionScriptPath(provisioningDir),
		Dynamic: true,
		Minor:   true,
	}
}

// GenerateIdempotencyCleanupInstruction generates a one-time instruction that performs a cleanup of the given key.
// Running this at the start of a new operation clears prior idempotency tracking under the key so commands
// associated with the key are re-evaluated.
func GenerateIdempotencyCleanupInstruction(provisioningDir, key string) planapi.OneTimeInstruction {
	if key == "" {
		return planapi.OneTimeInstruction{}
	}
	return planapi.OneTimeInstruction{
		CommonInstruction: planapi.CommonInstruction{
			Name:    "remove idempotency tracking",
			Command: "/bin/sh",
			Args: []string{
				"-c",
				fmt.Sprintf("rm -rf %s/idempotence/%s", provisioningDir, key),
			},
		},
	}
}

// IdempotentInstruction generates an idempotent action instruction that will execute the given command + args exactly once.
// It works by running a script that writes the given "value" to a file at <provisioningDir>/idempotence/<identifier>/<hashedCommand>,
// and checks this file to determine if it needs to run the instruction again. Notably, `identifier` must be a valid relative path.
func IdempotentInstruction(provisioningDir, identifier, value, command string, args []string, env []string) planapi.OneTimeInstruction {
	hashedCommand := planapi.PlanHash([]byte(command))
	hashedValue := planapi.PlanHash([]byte(value))
	return planapi.OneTimeInstruction{
		CommonInstruction: planapi.CommonInstruction{
			Name:    fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand),
			Command: "/bin/sh",
			Args: append([]string{
				"-x",
				IdempotentActionScriptPath(provisioningDir),
				strings.ToLower(identifier),
				hashedValue,
				hashedCommand,
				command,
				provisioningDir,
			}, args...),
			Env: env,
		},
	}
}

// ConvertToIdempotentInstruction converts a OneTimeInstruction to a OneTimeInstruction wrapped with the idempotent script.
// This is useful when an instruction may be used in various phases, without needing idempotency in all cases.
// identifier is expected to be a unique key for tracking, and value should be something like the generation of the attempt
// (and is what we track to determine whether we should run the instruction or not).
func ConvertToIdempotentInstruction(provisioningDir, identifier, value string, instruction planapi.OneTimeInstruction) planapi.OneTimeInstruction {
	newInstruction := IdempotentInstruction(provisioningDir, identifier, value, instruction.Command, instruction.Args, instruction.Env)
	newInstruction.Image = instruction.Image
	newInstruction.SaveOutput = instruction.SaveOutput
	return newInstruction
}
