package infrastructure

import (
	"testing"

	"github.com/doeshing/shai-go/internal/domain"
)

func TestGuardrailBlocksCriticalCommands(t *testing.T) {
	guardrail, err := NewGuardrail("")
	if err != nil {
		t.Fatalf("NewGuardrail error: %v", err)
	}

	result, err := guardrail.Evaluate("rm -rf /")
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	if result.Action != domain.ActionBlock || result.Level != domain.RiskCritical {
		t.Fatalf("expected critical block, got %+v", result)
	}
}

func TestGuardrailAllowsSafeCommand(t *testing.T) {
	guardrail, err := NewGuardrail("")
	if err != nil {
		t.Fatalf("NewGuardrail error: %v", err)
	}

	result, err := guardrail.Evaluate("ls -la")
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}

	if result.Level != domain.RiskSafe {
		t.Fatalf("expected safe, got %+v", result)
	}
}

func TestGuardrailProtectedPath(t *testing.T) {
	guardrail, err := NewGuardrail("")
	if err != nil {
		t.Fatalf("NewGuardrail error: %v", err)
	}
	result, err := guardrail.Evaluate("rm -rf /etc")
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	if result.Level == domain.RiskSafe {
		t.Fatalf("expected elevated risk for protected path, got %+v", result)
	}
	if len(result.PreviewEntries) == 0 {
		t.Log("No preview entries available for /etc on this system; continuing")
	}
}

func TestGuardrailWhitelist(t *testing.T) {
	guardrail, err := NewGuardrail("")
	if err != nil {
		t.Fatalf("NewGuardrail error: %v", err)
	}
	result, err := guardrail.Evaluate("ls")
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	if result.Level != domain.RiskSafe || result.Action != domain.ActionAllow {
		t.Fatalf("whitelisted command should be safe: %+v", result)
	}
}

func TestSuggestDryRunCommand(t *testing.T) {
	tests := []struct {
		command string
		expect  string
	}{
		{"kubectl apply -f deploy.yaml", "kubectl apply -f deploy.yaml --dry-run=client"},
		{"git commit -m 'msg'", "git status"},
		{"rm -rf tmp", "ls -rf tmp"},
		{"echo hi", ""},
	}
	for _, tt := range tests {
		if got := suggestDryRunCommand(tt.command); got != tt.expect {
			t.Fatalf("dryRun(%s)=%s want %s", tt.command, got, tt.expect)
		}
	}
}

func TestUndoHintsForCommand(t *testing.T) {
	hints := undoHintsForCommand("git push && kubectl apply")
	if len(hints) < 2 {
		t.Fatalf("expected multiple hints, got %v", hints)
	}
}
