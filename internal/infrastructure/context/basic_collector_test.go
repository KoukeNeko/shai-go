package contextcollector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/doeshing/shai-go/internal/domain"
)

func TestBasicCollectorIncludesFiles(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "file1.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := domain.Config{
		Context: domain.ContextSettings{
			IncludeFiles: true,
			MaxFiles:     5,
		},
	}

	collector := NewBasicCollector()
	snapshot, err := collector.Collect(context.Background(), cfg, domain.QueryRequest{})
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if len(snapshot.Files) == 0 {
		t.Fatal("expected file list to be populated")
	}
	if snapshot.Files[0].Path != "file1.txt" {
		t.Fatalf("expected file1.txt, got %+v", snapshot.Files[0])
	}
}

func TestBasicCollectorIncludesEnvWhenRequested(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	cfg := domain.Config{
		Context: domain.ContextSettings{
			IncludeFiles: false,
			MaxFiles:     5,
		},
	}
	collector := NewBasicCollector()
	req := domain.QueryRequest{WithEnv: true}
	snapshot, err := collector.Collect(context.Background(), cfg, req)
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if snapshot.EnvironmentVars["PATH"] == "" {
		t.Fatal("expected PATH to be present when WithEnv is true")
	}
}
