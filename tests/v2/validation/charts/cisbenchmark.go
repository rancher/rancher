package charts

const (
	Error                   = "error"
	Fail                    = "fail"
	scanPass                = "pass"
	scanFail                = "fail"
	CisConfigFileKey        = "cis"
	CISBenchmarkProjectName = "cis-operator-system"
	ClusterScanResourceType = "cis.cattle.io.clusterscan"
	ClusterScanReportType   = "cis.cattle.io.clusterscanreport"
)

// ClusterScanStatus represents the status field of a cluster scan object.
type ClusterScanStatus struct {
	Display Display
}

// Display contains the state of the cluster scan.
// State can be pending, running, reporting, pass, and fail
type Display struct {
	State   string
	Message string
}

// ClusterScanReportSpec represents the specification for a cluster scan report.
type ClusterScanReportSpec struct {
	ReportJson string
}

// CisReport is the report structure stored as report json in cluster scan report spec.
type CisReport struct {
	Total         int
	Pass          int
	Fail          int
	Skip          int
	Warn          int
	NotApplicable int
	Results       []*Group `json:"results"`
}

// Group is the result structure stored as report json in Results of CisReport
type Group struct {
	ID     string      `yaml:"id" json:"id"`
	Text   string      `json:"description"`
	Checks []*CisCheck `json:"checks"`
}

// CisCheck is the ID, Description and State structure of individual test in cluster scan.
type CisCheck struct {
	Id          string
	Description string
	State       string
}

type CisConfig struct {
	ProfileName  string `json:"profileName" yaml:"profileName"`
	ChartVersion string `json:"chartVersion" yaml:"chartVersion"`
}
