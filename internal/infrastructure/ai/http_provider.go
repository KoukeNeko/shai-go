// Package ai provides a unified HTTP-based AI provider implementation.
//
// This package implements a single, generic HTTP provider that supports any
// AI service through YAML configuration. Instead of hardcoding provider-specific
// logic, all API differences are handled through the APIFormat configuration:
//   - Request format (authentication, message structure)
//   - Response parsing (JSON path extraction)
//   - Provider-specific headers
//
// This eliminates the need for separate provider implementations and adapter patterns.
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

const providerName = "http"

// httpProvider is a configuration-driven HTTP-based AI provider.
// All provider-specific behavior is controlled through the model's APIFormat configuration.
type httpProvider struct {
	model      domain.ModelDefinition
	httpClient *http.Client
}

// newHTTPProvider creates a new HTTP-based AI provider.
func newHTTPProvider(model domain.ModelDefinition, client *http.Client) ports.Provider {
	return &httpProvider{
		model:      model,
		httpClient: client,
	}
}

func (p *httpProvider) Name() string {
	return providerName
}

func (p *httpProvider) Model() domain.ModelDefinition {
	return p.model
}

func (p *httpProvider) Generate(ctx context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	messages, err := renderPromptMessages(p.model, req.Prompt, req.Context)
	if err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("render prompt: %w", err)
	}

	requestBody, err := p.buildRequestBody(messages)
	if err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.model.Endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if err := p.setAuthHeaders(httpReq); err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("set auth headers: %w", err)
	}
	p.setExtraHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ports.ProviderResponse{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var responseBody bytes.Buffer
	if _, err := responseBody.ReadFrom(resp.Body); err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("read response body: %w", err)
	}

	content, err := p.parseResponse(responseBody.Bytes())
	if err != nil {
		return ports.ProviderResponse{}, fmt.Errorf("parse response: %w", err)
	}

	command := extractCommand(content)
	return ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: fmt.Sprintf("Generated via %s (%s)", p.model.Name, p.model.ModelID),
	}, nil
}

// buildRequestBody constructs the JSON request body based on the model's APIFormat configuration.
func (p *httpProvider) buildRequestBody(messages []domain.PromptMessage) ([]byte, error) {
	format := p.model.APIFormat

	request := map[string]interface{}{
		"model": p.model.ModelID,
	}

	if p.model.MaxTokens > 0 {
		request["max_tokens"] = p.model.MaxTokens
	}

	// Handle system messages based on configuration
	if format.IsSystemMessageSeparate() {
		systemPrompt, chatMessages := splitSystemMessages(messages, format)
		if systemPrompt != "" {
			request["system"] = systemPrompt
		}
		request["messages"] = chatMessages
	} else {
		// Inline mode: all messages in the messages array
		request["messages"] = formatMessagesInline(messages, format)
	}

	return json.Marshal(request)
}

// splitSystemMessages separates system messages from chat messages for providers
// that require system messages in a separate field (e.g., Anthropic).
func splitSystemMessages(messages []domain.PromptMessage, format domain.APIFormat) (string, []map[string]interface{}) {
	var systemLines []string
	var chatMessages []map[string]interface{}

	for _, msg := range messages {
		if strings.EqualFold(msg.Role, "system") {
			systemLines = append(systemLines, msg.Content)
			continue
		}
		chatMessages = append(chatMessages, formatMessage(msg, format))
	}

	return strings.TrimSpace(strings.Join(systemLines, "\n")), chatMessages
}

// formatMessagesInline formats all messages (including system) into the messages array.
func formatMessagesInline(messages []domain.PromptMessage, format domain.APIFormat) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		result = append(result, formatMessage(msg, format))
	}
	return result
}

// formatMessage formats a single message based on the content wrapper configuration.
func formatMessage(msg domain.PromptMessage, format domain.APIFormat) map[string]interface{} {
	message := map[string]interface{}{
		"role": strings.ToLower(msg.Role),
	}

	if format.IsContentWrapped() {
		// Anthropic format: wrap content in an array
		message["content"] = []map[string]string{
			{"type": "text", "text": msg.Content},
		}
	} else {
		// Standard format: direct string content
		message["content"] = msg.Content
	}

	return message
}

// setAuthHeaders configures authentication headers based on the model's APIFormat.
func (p *httpProvider) setAuthHeaders(req *http.Request) error {
	format := p.model.APIFormat
	apiKey := getAPIKey(p.model)

	if apiKey == "" {
		return fmt.Errorf("missing API key: set %s environment variable", p.model.AuthEnvVar)
	}

	headerName := format.GetAuthHeaderName()
	headerPrefix := format.GetAuthHeaderPrefix()
	headerValue := headerPrefix + apiKey

	req.Header.Set(headerName, headerValue)

	// Handle optional organization header for OpenAI
	if p.model.OrgEnvVar != "" {
		if orgID := os.Getenv(p.model.OrgEnvVar); orgID != "" {
			req.Header.Set("OpenAI-Organization", orgID)
		}
	}

	return nil
}

// setExtraHeaders adds any additional headers defined in the APIFormat configuration.
func (p *httpProvider) setExtraHeaders(req *http.Request) {
	for key, value := range p.model.APIFormat.ExtraHeaders {
		req.Header.Set(key, value)
	}
}

// parseResponse extracts the generated text from the JSON response using the configured JSON path.
func (p *httpProvider) parseResponse(body []byte) (string, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("unmarshal JSON: %w", err)
	}

	path := p.model.APIFormat.GetResponseJSONPath()
	content, err := extractJSONPath(response, path)
	if err != nil {
		return "", fmt.Errorf("extract from path '%s': %w", path, err)
	}

	return strings.TrimSpace(content), nil
}

// extractJSONPath extracts a string value from a nested JSON structure using a simple path notation.
// Supported paths: "field", "field.nested", "field[0]", "field[0].nested.field"
func extractJSONPath(data map[string]interface{}, path string) (string, error) {
	parts := parseJSONPath(path)
	var current interface{} = data

	for _, part := range parts {
		switch part.kind {
		case "field":
			obj, ok := current.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("expected object at '%s'", part.value)
			}
			var found bool
			current, found = obj[part.value]
			if !found {
				return "", fmt.Errorf("field '%s' not found", part.value)
			}

		case "index":
			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("expected array at index %s", part.value)
			}
			var idx int
			fmt.Sscanf(part.value, "%d", &idx)
			if idx < 0 || idx >= len(arr) {
				return "", fmt.Errorf("index %d out of bounds (len=%d)", idx, len(arr))
			}
			current = arr[idx]
		}
	}

	// Final value should be a string
	if str, ok := current.(string); ok {
		return str, nil
	}

	return "", fmt.Errorf("final value is not a string: %T", current)
}

type pathPart struct {
	kind  string // "field" or "index"
	value string
}

// parseJSONPath converts "content[0].text" into structured path parts.
// Examples:
//   - "content[0].text" → [{field, "content"}, {index, "0"}, {field, "text"}]
//   - "choices[0].message.content" → [{field, "choices"}, {index, "0"}, {field, "message"}, {field, "content"}]
func parseJSONPath(path string) []pathPart {
	var parts []pathPart
	current := ""

	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch ch {
		case '.':
			if current != "" {
				parts = append(parts, pathPart{kind: "field", value: current})
				current = ""
			}
		case '[':
			if current != "" {
				parts = append(parts, pathPart{kind: "field", value: current})
				current = ""
			}
			// Find closing ]
			j := i + 1
			for j < len(path) && path[j] != ']' {
				j++
			}
			if j < len(path) {
				parts = append(parts, pathPart{kind: "index", value: path[i+1 : j]})
				i = j
			}
		default:
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, pathPart{kind: "field", value: current})
	}

	return parts
}

// extractCommand attempts to extract a shell command from the AI response.
// It tries multiple extraction strategies: code blocks, command prefix, raw text.
func extractCommand(content string) string {
	if code := extractCodeBlock(content); code != "" {
		return code
	}
	if cmd := extractCommandLine(content); cmd != "" {
		return cmd
	}
	return strings.TrimSpace(content)
}

// extractCodeBlock finds and extracts the first markdown code block (```...```).
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
	// Remove language marker (sh, bash, etc.) if present
	if len(lines) > 0 && (strings.HasPrefix(lines[0], "sh") || strings.HasPrefix(lines[0], "bash")) {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// extractCommandLine looks for lines prefixed with "command:" and extracts the text after it.
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

// getAPIKey retrieves the API key from environment variables.
func getAPIKey(model domain.ModelDefinition) string {
	if model.AuthEnvVar != "" {
		if key := os.Getenv(model.AuthEnvVar); key != "" {
			return key
		}
	}
	return ""
}
