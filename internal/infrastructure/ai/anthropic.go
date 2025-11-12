package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

type anthropicProvider struct {
	model      domain.ModelDefinition
	httpClient *http.Client
}

func newAnthropicProvider(model domain.ModelDefinition, client *http.Client) ports.Provider {
	return &anthropicProvider{
		model:      model,
		httpClient: client,
	}
}

func (p *anthropicProvider) Name() string {
	return "anthropic"
}

func (p *anthropicProvider) Model() domain.ModelDefinition {
	return p.model
}

func (p *anthropicProvider) Generate(ctx context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	apiKey := resolveAuth(p.model.AuthEnvVar, "ANTHROPIC_API_KEY")
	if apiKey == "" {
		return newHeuristicProvider(p.model).Generate(ctx, req)
	}

	payload := anthropicRequest{
		Model:     valueOrDefault(p.model.ModelID, "claude-3-5-sonnet-20240620"),
		MaxTokens: valueOrDefaultInt(p.model.MaxTokens, 1024),
		System:    renderSystemPrompt(p.model, req.Context),
		Messages: []anthropicMessage{
			{
				Role: "user",
				Content: []anthropicContent{
					{Type: "text", Text: renderUserPrompt(req.Prompt, req.Context)},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, valueOrDefault(p.model.Endpoint, "https://api.anthropic.com/v1/messages"), bytes.NewReader(body))
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ports.ProviderResponse{}, fmt.Errorf("anthropic: %s", resp.Status)
	}

	var decoded anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ports.ProviderResponse{}, err
	}

	content := decoded.FirstText()
	command := extractCommand(content)
	return ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: "Generated via Claude",
	}, nil
}

func renderSystemPrompt(model domain.ModelDefinition, ctx domain.ContextSnapshot) string {
	if len(model.Prompt) == 0 {
		return fmt.Sprintf("You are SHAI, a cautious shell assistant operating in %s.", ctx.WorkingDir)
	}
	var builder strings.Builder
	for _, msg := range model.Prompt {
		if msg.Role == "system" {
			builder.WriteString(msg.Content)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func renderUserPrompt(prompt string, ctx domain.ContextSnapshot) string {
	var builder strings.Builder
	builder.WriteString("User query:\n")
	builder.WriteString(prompt)
	builder.WriteString("\n\nContext:\n")
	builder.WriteString(fmt.Sprintf("- Directory: %s\n", ctx.WorkingDir))
	builder.WriteString(fmt.Sprintf("- Shell: %s\n", ctx.Shell))
	builder.WriteString(fmt.Sprintf("- OS: %s\n", ctx.OS))
	if len(ctx.AvailableTools) > 0 {
		builder.WriteString(fmt.Sprintf("- Tools: %s\n", strings.Join(ctx.AvailableTools, ", ")))
	}
	if ctx.Git != nil {
		builder.WriteString(fmt.Sprintf("- Git: branch %s, modified %d, untracked %d\n", ctx.Git.Branch, ctx.Git.ModifiedCount, ctx.Git.UntrackedCount))
	}
	if ctx.Kubernetes != nil {
		builder.WriteString(fmt.Sprintf("- K8s: context %s namespace %s\n", ctx.Kubernetes.Context, ctx.Kubernetes.Namespace))
	}
	builder.WriteString("\nReturn ONLY the shell command and short reasoning.")
	return builder.String()
}

func extractCommand(content string) string {
	if strings.Contains(content, "```") {
		start := strings.Index(content, "```")
		suffix := content[start+3:]
		if end := strings.Index(suffix, "```"); end != -1 {
			block := suffix[:end]
			lines := strings.Split(block, "\n")
			if len(lines) > 0 && strings.HasPrefix(lines[0], "sh") {
				lines = lines[1:]
			}
			return strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "command:") {
			return strings.TrimSpace(line[len("command:"):])
		}
	}
	return strings.TrimSpace(content)
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func (a anthropicResponse) FirstText() string {
	if len(a.Content) == 0 {
		return ""
	}
	return a.Content[0].Text
}

func resolveAuth(primary string, fallback string) string {
	if primary != "" {
		if value := os.Getenv(primary); value != "" {
			return value
		}
	}
	return os.Getenv(fallback)
}

func valueOrDefault(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func valueOrDefaultInt(value int, def int) int {
	if value == 0 {
		return def
	}
	return value
}
