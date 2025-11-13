package ai

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/doeshing/shai-go/internal/domain"
)

// renderPromptMessages expands model prompt templates with context data and ensures a user message exists.
func renderPromptMessages(model domain.ModelDefinition, userPrompt string, ctx domain.ContextSnapshot) ([]domain.PromptMessage, error) {
	data := buildTemplateData(userPrompt, ctx)
	messages := model.Prompt
	if len(messages) == 0 {
		messages = defaultTemplateMessages()
	}

	rendered := make([]domain.PromptMessage, 0, len(messages))
	for _, msg := range messages {
		content, err := executeTemplate(msg.Content, data)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, domain.PromptMessage{
			Role:    msg.Role,
			Content: strings.TrimSpace(content),
		})
	}

	if !hasUserMessage(rendered) {
		fallback, err := executeTemplate("{{.Prompt}}", data)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, domain.PromptMessage{
			Role:    "user",
			Content: strings.TrimSpace(fallback),
		})
	}

	return rendered, nil
}

type templateData struct {
	Prompt         string
	WorkingDir     string
	Shell          string
	OS             string
	User           string
	Files          string
	AvailableTools string
	GitStatus      string
	K8sContext     string
	K8sNamespace   string
	Environment    string
}

func buildTemplateData(prompt string, ctx domain.ContextSnapshot) templateData {
	return templateData{
		Prompt:         fmt.Sprintf("%s\n\n%s", strings.TrimSpace(prompt), contextSnippet(ctx)),
		WorkingDir:     ctx.WorkingDir,
		Shell:          ctx.Shell,
		OS:             ctx.OS,
		User:           ctx.User,
		Files:          filesSummary(ctx.Files),
		AvailableTools: strings.Join(ctx.AvailableTools, ", "),
		GitStatus:      gitSummary(ctx.Git),
		K8sContext:     kubeContext(ctx.Kubernetes),
		K8sNamespace:   kubeNamespace(ctx.Kubernetes),
		Environment:    envSummary(ctx.EnvironmentVars),
	}
}

func contextSnippet(ctx domain.ContextSnapshot) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Directory: %s", ctx.WorkingDir))
	if ctx.Shell != "" {
		lines = append(lines, fmt.Sprintf("Shell: %s", ctx.Shell))
	}
	if ctx.OS != "" {
		lines = append(lines, fmt.Sprintf("OS: %s", ctx.OS))
	}
	if tools := strings.Join(ctx.AvailableTools, ", "); tools != "" {
		lines = append(lines, fmt.Sprintf("Available tools: %s", tools))
	}
	if summary := gitSummary(ctx.Git); summary != "" {
		lines = append(lines, fmt.Sprintf("Git: %s", summary))
	}
	if ns := kubeNamespace(ctx.Kubernetes); ns != "" {
		lines = append(lines, fmt.Sprintf("Kubernetes: %s (%s)", ns, kubeContext(ctx.Kubernetes)))
	}
	if files := filesSummary(ctx.Files); files != "" {
		lines = append(lines, fmt.Sprintf("Files: %s", files))
	}
	return strings.Join(lines, "\n")
}

func filesSummary(files []domain.FileInfo) string {
	if len(files) == 0 {
		return ""
	}
	var names []string
	for _, file := range files {
		names = append(names, file.Path)
	}
	return strings.Join(names, ", ")
}

func gitSummary(status *domain.GitStatus) string {
	if status == nil {
		return ""
	}
	return fmt.Sprintf("branch %s, modified %d, untracked %d", status.Branch, status.ModifiedCount, status.UntrackedCount)
}

func kubeNamespace(kube *domain.KubeStatus) string {
	if kube == nil {
		return ""
	}
	return kube.Namespace
}

func kubeContext(kube *domain.KubeStatus) string {
	if kube == nil {
		return ""
	}
	return kube.Context
}

func envSummary(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	var keys []string
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, env[key]))
	}
	return strings.Join(parts, ", ")
}

func executeTemplate(raw string, data templateData) (string, error) {
	tmpl, err := template.New("prompt").Parse(raw)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func hasUserMessage(messages []domain.PromptMessage) bool {
	for _, msg := range messages {
		if strings.EqualFold(msg.Role, "user") {
			return true
		}
	}
	return false
}

func defaultTemplateMessages() []domain.PromptMessage {
	return []domain.PromptMessage{
		{
			Role: "system",
			Content: `You are SHAI, a cautious shell assistant.
Always output a single shell command (with optional short explanation).
Current environment:
- Directory: {{.WorkingDir}}
- Shell: {{.Shell}}
- OS: {{.OS}}
{{if .AvailableTools}}- Tools: {{.AvailableTools}}{{end}}
{{if .GitStatus}}- Git: {{.GitStatus}}{{end}}
{{if .K8sNamespace}}- Kubernetes: {{.K8sContext}}/{{.K8sNamespace}}{{end}}`,
		},
		{
			Role:    "user",
			Content: "{{.Prompt}}",
		},
	}
}

