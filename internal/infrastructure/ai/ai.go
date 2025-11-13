// Package ai provides AI provider factory and HTTP-based provider implementation.
//
// This package implements a unified, configuration-driven approach to AI providers:
//   - Factory: Creates provider instances based on model definitions
//   - HTTP Provider: Generic HTTP client supporting any AI service via YAML config
//   - Prompt Templates: Renders user prompts with context using Go templates
//
// All provider-specific behavior is controlled through the model's APIFormat configuration,
// eliminating the need for separate provider implementations and adapter patterns.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

const (
	httpClientTimeout = 60 * time.Second
	providerName      = "http"
)

// ====================================================================================
// Factory
// ====================================================================================

// Factory creates AI provider instances based on model definitions.
// It maintains a single HTTP client shared across all providers.
type Factory struct {
	httpClient *http.Client
}

// NewFactory creates a new provider factory with a configured HTTP client.
func NewFactory() *Factory {
	return &Factory{
		httpClient: &http.Client{Timeout: httpClientTimeout},
	}
}

// ForModel creates a generic HTTP provider for any model definition.
// All provider-specific behavior is controlled through the model's APIFormat configuration.
func (f *Factory) ForModel(model domain.ModelDefinition) (ports.Provider, error) {
	return newHTTPProvider(model, f.httpClient), nil
}

var _ ports.ProviderFactory = (*Factory)(nil)

// ====================================================================================
// HTTP Provider
// ====================================================================================

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

// ====================================================================================
// Prompt Template Rendering
// ====================================================================================

// renderPromptMessages expands model prompt templates with context data and ensures a user message exists.
// If the model has no custom prompt template, it uses a sensible default system prompt.
//
// Template Variables Available:
//   - {{.Prompt}}: User's input prompt with context snippet
//   - {{.WorkingDir}}: Current working directory
//   - {{.Shell}}: Active shell (bash, zsh, etc.)
//   - {{.OS}}: Operating system
//   - {{.Files}}: Comma-separated list of relevant files
//   - {{.AvailableTools}}: Comma-separated list of available CLI tools
//   - {{.GitStatus}}: Git repository status summary
//   - {{.K8sContext}}: Kubernetes context name
//   - {{.K8sNamespace}}: Kubernetes namespace
//   - {{.Environment}}: Environment variables as key=value pairs
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
