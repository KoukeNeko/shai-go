package contextcollector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

// BasicCollector implements ContextCollector with filesystem + tool detection.
type BasicCollector struct {
	toolsToCheck []string
}

func NewBasicCollector() *BasicCollector {
	return &BasicCollector{
		toolsToCheck: []string{"docker", "kubectl", "git", "npm", "yarn", "pnpm", "python", "python3", "go", "node", "cargo", "make"},
	}
}

// Collect gathers context data.
func (c *BasicCollector) Collect(ctx context.Context, cfg domain.Config, req domain.QueryRequest) (domain.ContextSnapshot, error) {
	wd, _ := os.Getwd()
	shell := detectShell()
	user := os.Getenv("USER")

	files := []domain.FileInfo{}
	if cfg.Context.IncludeFiles {
		files = listFiles(wd, cfg.Context.MaxFiles)
	}

	tools := c.detectTools()
	var gitStatus *domain.GitStatus
	if shouldCollect(cfg.Context.IncludeGit) {
		if status := collectGitInfo(ctx, wd); status != nil {
			gitStatus = status
		}
	}

	var kubeStatus *domain.KubeStatus
	if shouldCollect(cfg.Context.IncludeK8s) {
		if status := collectKubeInfo(ctx); status != nil {
			kubeStatus = status
		}
	}

	envVars := map[string]string{}
	if cfg.Context.IncludeEnv || req.WithEnv {
		envVars["PATH"] = os.Getenv("PATH")
		if kubeConfig := os.Getenv("KUBECONFIG"); kubeConfig != "" {
			envVars["KUBECONFIG"] = kubeConfig
		}
	}

	return domain.ContextSnapshot{
		WorkingDir:      wd,
		Shell:           shell,
		OS:              runtime.GOOS,
		User:            user,
		Files:           files,
		AvailableTools:  tools,
		Git:             gitStatus,
		Kubernetes:      kubeStatus,
		EnvironmentVars: envVars,
	}, nil
}

func (c *BasicCollector) detectTools() []string {
	var available []string
	for _, tool := range c.toolsToCheck {
		if _, err := exec.LookPath(tool); err == nil {
			available = append(available, tool)
		}
	}
	sort.Strings(available)
	return available
}

func listFiles(dir string, limit int) []domain.FileInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []domain.FileInfo
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if len(files) >= limit {
			break
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, domain.FileInfo{
			Path: entry.Name(),
			Size: info.Size(),
			Type: toFileType(info),
		})
	}
	return files
}

func toFileType(info os.FileInfo) domain.FileType {
	switch {
	case info.Mode().IsDir():
		return domain.FileTypeDir
	case info.Mode()&os.ModeSymlink != 0:
		return domain.FileTypeSymlink
	case info.Mode().IsRegular():
		return domain.FileTypeFile
	default:
		return domain.FileTypeUnknown
	}
}

func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	return "unknown"
}

func shouldCollect(setting string) bool {
	switch strings.ToLower(setting) {
	case "always":
		return true
	case "never":
		return false
	default:
		return true
	}
}

func collectGitInfo(ctx context.Context, dir string) *domain.GitStatus {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil
	}
	branch := runCmd(ctx, dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	statusShort := runCmd(ctx, dir, "git", "status", "--short")
	modified := 0
	untracked := 0
	for _, line := range strings.Split(statusShort, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untracked++
		} else {
			modified++
		}
	}
	return &domain.GitStatus{
		Branch:         strings.TrimSpace(branch),
		ModifiedCount:  modified,
		UntrackedCount: untracked,
		Summary:        strings.TrimSpace(statusShort),
	}
}

func collectKubeInfo(ctx context.Context) *domain.KubeStatus {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil
	}
	contextName := strings.TrimSpace(runCmd(ctx, "", "kubectl", "config", "current-context"))
	namespace := strings.TrimSpace(runCmd(ctx, "", "kubectl", "config", "view", "--minify", "--output", "jsonpath={..namespace}"))
	return &domain.KubeStatus{
		Context:   contextName,
		Namespace: namespace,
	}
}

func runCmd(ctx context.Context, dir string, name string, args ...string) string {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

var _ ports.ContextCollector = (*BasicCollector)(nil)
