package pyexecclient

type AnalyzeRequest struct {
	RequestID      string
	TenantID       uint64
	ProjectID      uint64
	SessionID      uint64
	UserID         uint64
	Question       string
	Mode           string
	AnalysisGoal   string
	TemplateName   string
	TemplateParams map[string]any
	Datasets       []Dataset
	Limits         Limits
}

type Dataset struct {
	Name           string
	DatasourceID   uint64
	DatasourceName string
	Purpose        string
	ExecutionID    uint64
	Columns        []string
	Rows           []map[string]any
}

type Limits struct {
	TimeoutMS      int
	MaxInputRows   int
	MaxOutputRows  int
	MaxStdoutChars int
}

type AnalyzeResponse struct {
	Success        bool
	Summary        string
	Observation    string
	Tables         []Table
	Charts         []Chart
	Metrics        []Metric
	Warnings       []string
	StdoutPreview  string
	StderrPreview  string
	Error          string
	DurationMS     int64
	InputRowCount  int
	OutputRowCount int
	AnalysisKind   string
	CodeHash       string
	TemplateName   string
}

type Table struct {
	Name    string
	Columns []string
	Rows    []map[string]any
}

type Chart struct {
	Type       string
	Title      string
	XField     string
	YFields    []string
	NameField  string
	ValueField string
	Reason     string
	Rows       []map[string]any
}

type Metric struct {
	Name    string
	Label   string
	Value   float64
	Unit    string
	Display string
}

type HealthStatus struct {
	OK           bool
	Version      string
	Capabilities map[string]bool
}
