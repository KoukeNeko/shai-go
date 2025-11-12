package domain

import "context"

// QueryRequest captures user intent originating from CLI or shell integration.
type QueryRequest struct {
	Context         context.Context
	Prompt          string
	ModelOverride   string
	PreviewOnly     bool
	AutoExecute     bool
	CopyToClipboard bool
	WithGitStatus   bool
	WithEnv         bool
	WithK8sInfo     bool
	Debug           bool
}

// QueryResponse is the canonical response propagated back to the CLI.
type QueryResponse struct {
	Command            string
	NaturalLanguage    string
	Reasoning          string
	RiskAssessment     RiskAssessment
	ExecutionPlanned   bool
	ExecutionResult    *ExecutionResult
	ContextInformation ContextSnapshot
	FromCache          bool
}

// ExecutionResult wraps details from the command executor.
type ExecutionResult struct {
	Ran         bool
	Stdout      string
	Stderr      string
	ExitCode    int
	DurationMS  int64
	Err         error
	DryRunNotes string
}

// QueryService exposes the use-case boundary for handling a query.
type QueryService interface {
	Run(QueryRequest) (QueryResponse, error)
}
