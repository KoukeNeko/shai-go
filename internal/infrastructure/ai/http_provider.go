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

type httpProvider struct {
	name       string
	model      domain.ModelDefinition
	httpClient *http.Client
	adapter    providerAdapter
}

type providerAdapter struct {
	buildRequest  func(domain.ModelDefinition, []domain.PromptMessage) ([]byte, error)
	parseResponse func([]byte) (string, error)
	setHeaders    func(*http.Request, domain.ModelDefinition) error
}

func newHTTPProvider(name string, model domain.ModelDefinition, client *http.Client, adapter providerAdapter) ports.Provider {
	return &httpProvider{
		name:       name,
		model:      model,
		httpClient: client,
		adapter:    adapter,
	}
}

func (p *httpProvider) Name() string {
	return p.name
}

func (p *httpProvider) Model() domain.ModelDefinition {
	return p.model
}

func (p *httpProvider) Generate(ctx context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	messages, err := renderPromptMessages(p.model, req.Prompt, req.Context)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	requestBody, err := p.adapter.buildRequest(p.model, messages)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	endpoint := p.model.Endpoint
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	httpReq.Header.Set("content-type", "application/json")
	if err := p.adapter.setHeaders(httpReq, p.model); err != nil {
		return ports.ProviderResponse{}, err
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ports.ProviderResponse{}, fmt.Errorf("%s: %s", p.name, resp.Status)
	}

	var responseBody bytes.Buffer
	if _, err := responseBody.ReadFrom(resp.Body); err != nil {
		return ports.ProviderResponse{}, err
	}

	content, err := p.adapter.parseResponse(responseBody.Bytes())
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	command := extractCommand(content)
	return ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: fmt.Sprintf("Generated via %s", p.name),
	}, nil
}

func anthropicAdapter() providerAdapter {
	return providerAdapter{
		buildRequest:  buildAnthropicRequest,
		parseResponse: parseAnthropicResponse,
		setHeaders:    setAnthropicHeaders,
	}
}

func openaiAdapter() providerAdapter {
	return providerAdapter{
		buildRequest:  buildChatCompletionRequest,
		parseResponse: parseChatCompletionResponse,
		setHeaders:    setOpenAIHeaders,
	}
}

func ollamaAdapter() providerAdapter {
	return providerAdapter{
		buildRequest:  buildChatCompletionRequest,
		parseResponse: parseChatCompletionResponse,
		setHeaders:    setOllamaHeaders,
	}
}

func buildAnthropicRequest(model domain.ModelDefinition, messages []domain.PromptMessage) ([]byte, error) {
	systemPrompt, chatMessages := splitSystemMessages(messages)

	request := map[string]interface{}{
		"model":      defaultString(model.ModelID, "claude-3-5-sonnet-20240620"),
		"max_tokens": defaultInt(model.MaxTokens, 1024),
		"messages":   chatMessages,
	}
	if systemPrompt != "" {
		request["system"] = systemPrompt
	}

	return json.Marshal(request)
}

func splitSystemMessages(messages []domain.PromptMessage) (string, []map[string]interface{}) {
	var systemLines []string
	var chatMessages []map[string]interface{}

	for _, msg := range messages {
		if strings.EqualFold(msg.Role, "system") {
			systemLines = append(systemLines, msg.Content)
			continue
		}
		chatMessages = append(chatMessages, map[string]interface{}{
			"role": msg.Role,
			"content": []map[string]string{
				{"type": "text", "text": msg.Content},
			},
		})
	}

	return strings.TrimSpace(strings.Join(systemLines, "\n")), chatMessages
}

func parseAnthropicResponse(body []byte) (string, error) {
	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Content) == 0 {
		return "", nil
	}
	return response.Content[0].Text, nil
}

func setAnthropicHeaders(req *http.Request, model domain.ModelDefinition) error {
	apiKey := getEnv(model.AuthEnvVar, "ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("missing API key: set %s or ANTHROPIC_API_KEY", model.AuthEnvVar)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	return nil
}

func buildChatCompletionRequest(model domain.ModelDefinition, messages []domain.PromptMessage) ([]byte, error) {
	chatMessages := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		chatMessages = append(chatMessages, map[string]string{
			"role":    strings.ToLower(msg.Role),
			"content": msg.Content,
		})
	}

	request := map[string]interface{}{
		"model":    model.ModelID,
		"messages": chatMessages,
	}
	if model.MaxTokens > 0 {
		request["max_tokens"] = model.MaxTokens
	}

	return json.Marshal(request)
}

func parseChatCompletionResponse(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func setOpenAIHeaders(req *http.Request, model domain.ModelDefinition) error {
	apiKey := getEnv(model.AuthEnvVar, "OPENAI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("missing API key: set %s or OPENAI_API_KEY", model.AuthEnvVar)
	}
	req.Header.Set("authorization", "Bearer "+apiKey)

	if org := getEnv(model.OrgEnvVar, "OPENAI_ORG_ID"); org != "" {
		req.Header.Set("OpenAI-Organization", org)
	}
	return nil
}

func setOllamaHeaders(req *http.Request, model domain.ModelDefinition) error {
	return nil
}

func extractCommand(content string) string {
	if code := extractCodeBlock(content); code != "" {
		return code
	}
	if cmd := extractCommandLine(content); cmd != "" {
		return cmd
	}
	return strings.TrimSpace(content)
}

func extractCodeBlock(content string) string {
	if !strings.Contains(content, "```") {
		return ""
	}

	start := strings.Index(content, "```")
	suffix := content[start+3:]
	end := strings.Index(suffix, "```")
	if end == -1 {
		return ""
	}

	block := suffix[:end]
	lines := strings.Split(block, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "sh") {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractCommandLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "command:") {
			return strings.TrimSpace(line[len("command:"):])
		}
	}
	return ""
}

func getEnv(primary, fallback string) string {
	if primary != "" {
		if value := os.Getenv(primary); value != "" {
			return value
		}
	}
	if fallback != "" {
		return os.Getenv(fallback)
	}
	return ""
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
