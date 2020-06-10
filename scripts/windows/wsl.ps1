$job = Start-Job { wsl -- CATTLE_DEV_MODE=30 KUBECONFIG=/home/ubuntu/.config/k3d/rancher/kubeconfig.yaml rancher --debug }

[console]::TreatControlCAsInput = $true
while ($true) {
    if ([console]::KeyAvailable) {
        $key = [system.console]::readkey($true)
        if (($key.modifiers -band [consolemodifiers]"control") -and ($key.key -eq "C")) {
        	"Terminating..."
        	Stop-Job $job
        	Exit
        }
    }
}

Exit