package planner

import (
	"fmt"
	"path"
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
)

const idempotentActionScript = `
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

func idempotentActionScriptPath(controlPlane *rkev1.RKEControlPlane) string {
	return path.Join(capr.GetProvisioningDataDir(&controlPlane.Spec.RKEClusterSpecCommon), "idempotence/idempotent.sh")
}

// generateIdempotencyCleanupInstruction generates a one-time instruction that performs a cleanup of the given key.
func generateIdempotencyCleanupInstruction(controlPlane *rkev1.RKEControlPlane, key string) plan.OneTimeInstruction {
	if key == "" {
		return plan.OneTimeInstruction{}
	}
	return plan.OneTimeInstruction{
		Name:    "remove idempotency tracking",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf("rm -rf %s/idempotence/%s", capr.GetProvisioningDataDir(&controlPlane.Spec.RKEClusterSpecCommon), key),
		},
	}
}

// idempotentInstruction generates an idempotent action instruction that will execute the given command + args exactly once.
// It works by running a script that writes the given "value" to a file at /var/lib/rancher/capr/idempotence/<identifier>/<hashedCommand>,
// and checks this file to determine if it needs to run the instruction again. Notably, `identifier` must be a valid relative path.
func idempotentInstruction(controlPlane *rkev1.RKEControlPlane, identifier, value, command string, args []string, env []string) plan.OneTimeInstruction {
	hashedCommand := PlanHash([]byte(command))
	hashedValue := PlanHash([]byte(value))
	return plan.OneTimeInstruction{
		Name:    fmt.Sprintf("idempotent-%s-%s-%s", identifier, hashedValue, hashedCommand),
		Command: "/bin/sh",
		Args: append([]string{
			"-x",
			idempotentActionScriptPath(controlPlane),
			strings.ToLower(identifier),
			hashedValue,
			hashedCommand,
			command,
			capr.GetProvisioningDataDir(&controlPlane.Spec.RKEClusterSpecCommon)},
			args...),
		Env: env,
	}
}

// convertToIdempotentInstruction converts a OneTimeInstruction to a OneTimeInstruction wrapped with the idempotent script.
// This is useful when an instruction may be used in various phases, without needing idempotency in all cases.
// identifier is expected to be a unique key for tracking, and value should be something like the generation of the attempt
// (and is what we track to determine whether we should run the instruction or not)
func convertToIdempotentInstruction(controlPlane *rkev1.RKEControlPlane, identifier, value string, instruction plan.OneTimeInstruction) plan.OneTimeInstruction {
	newInstruction := idempotentInstruction(controlPlane, identifier, value, instruction.Command, instruction.Args, instruction.Env)
	newInstruction.Image = instruction.Image
	newInstruction.SaveOutput = instruction.SaveOutput
	return newInstruction
}

// idempotentRestartInstructions generates an idempotent restart instructions for the given runtimeUnit. It checks the
// unit for failure, resets it if necessary, and restarts the unit. identifier is expected to be a unique key for tracking,
// and value should be something like the generation of the attempt (and is what we track to determine whether we should run the instruction or not)
func idempotentRestartInstructions(controlPlane *rkev1.RKEControlPlane, identifier, value, runtimeUnit string) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		idempotentInstruction(
			controlPlane,
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
			controlPlane,
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

// idempotentStopInstruction generates an idempotent stop instruction for the given runtimeUnit. It simply calls systemctl stop <runtime-unit>
// identifier is expected to be a unique key for tracking, and value should be something like the generation of the attempt (and is what we track to determine whether we should run the instruction or not)
func idempotentStopInstruction(controlPlane *rkev1.RKEControlPlane, identifier, value, runtimeUnit string) plan.OneTimeInstruction {
	return idempotentInstruction(
		controlPlane,
		identifier+"-stop",
		value,
		"systemctl",
		[]string{
			"stop",
			runtimeUnit,
		},
		[]string{},
	)
}
