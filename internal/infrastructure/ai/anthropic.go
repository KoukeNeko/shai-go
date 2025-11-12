package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	rendered, err := renderPromptMessages(p.model, req.Prompt, req.Context)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	systemPrompt, chatMessages := splitSystemMessages(rendered)

	payload := anthropicRequest{
		Model:     valueOrDefault(p.model.ModelID, "claude-3-5-sonnet-20240620"),
		MaxTokens: valueOrDefaultInt(p.model.MaxTokens, 1024),
		System:    systemPrompt,
		Messages:  chatMessages,
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
	result := ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: "Generated via Claude",
	}
	emitStream(req, content)
	return result, nil
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

func splitSystemMessages(messages []domain.PromptMessage) (string, []anthropicMessage) {
	var systemLines []string
	var chat []anthropicMessage
	for _, msg := range messages {
		if strings.EqualFold(msg.Role, "system") {
			systemLines = append(systemLines, msg.Content)
			continue
		}
		chat = append(chat, anthropicMessage{
			Role: msg.Role,
			Content: []anthropicContent{
				{Type: "text", Text: msg.Content},
			},
		})
	}
	return strings.TrimSpace(strings.Join(systemLines, "\n")), chat
}
