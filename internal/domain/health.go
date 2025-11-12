package domain

// HealthStatus indicates doctor check outcomes.
type HealthStatus string

const (
	HealthOK    HealthStatus = "ok"
	HealthWarn  HealthStatus = "warn"
	HealthError HealthStatus = "error"
)

// HealthCheck captures a single diagnostic result.
type HealthCheck struct {
	Name    string
	Status  HealthStatus
	Details string
}

// HealthReport aggregates checks.
type HealthReport struct {
	Checks []HealthCheck
}
