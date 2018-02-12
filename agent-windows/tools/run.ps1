param(
    [string]$RegisterUrl=$(throw "Error missing: -RegisterUrl <url>"),
    [string]$HostLabels,
    [string]$AgentIp
)

$rancherBaseDir="rancher"

$tools=[System.Environment]::GetEnvironmentVariable("TOOLS_LIST").Split(",")
foreach ($copyTarget in $tools) {
    Copy-item -Path "c:/program files/$copyTarget" -Destination "c:/program files/$rancherBaseDir/$copyTarget"
}
Write-Output "`"$RegisterUrl`",`"$HostLabels`",`"$AgentIp`""