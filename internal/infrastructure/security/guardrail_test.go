package security

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
