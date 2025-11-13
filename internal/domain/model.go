// Package domain defines core business entities and value objects for SHAI.
//
// This file contains AI model and provider definitions used throughout the application.
// The domain layer is independent of infrastructure concerns and represents pure
// business logic and data structures.
package domain

// ModelDefinition describes an AI provider configuration declared in the config file.
// Each model represents a specific AI service endpoint with its authentication and
// generation parameters.
type ModelDefinition struct {
	Name       string          `yaml:"name"`
	Endpoint   string          `yaml:"endpoint"`
	AuthEnvVar string          `yaml:"auth_env_var"`
	OrgEnvVar  string          `yaml:"org_env_var"`
	ModelID    string          `yaml:"model_id"`
	MaxTokens  int             `yaml:"max_tokens"`
	Prompt     []PromptMessage `yaml:"prompt"`
	APIFormat  APIFormat       `yaml:"api_format,omitempty"`
}

// APIFormat defines how to construct requests and parse responses for different AI APIs.
// All fields are optional with sensible defaults (OpenAI-compatible format).
type APIFormat struct {
	// AuthHeaderName specifies the HTTP header name for authentication.
	// Default: "Authorization"
	AuthHeaderName string `yaml:"auth_header_name,omitempty"`

	// AuthHeaderPrefix is prepended to the API key value.
	// Default: "Bearer " (with trailing space)
	// Set to empty string for providers that don't use a prefix (e.g., Anthropic's "x-api-key")
	AuthHeaderPrefix string `yaml:"auth_header_prefix,omitempty"`

	// SystemMessageMode controls how system messages are sent to the API.
	// Values: "inline" (default) - system messages in the messages array
	//         "separate" - system messages in a separate "system" field (Anthropic)
	SystemMessageMode string `yaml:"system_message_mode,omitempty"`

	// ContentWrapper controls how message content is formatted.
	// Values: "standard" (default) - direct string content
	//         "anthropic" - wrap in [{"type": "text", "text": "..."}] array
	ContentWrapper string `yaml:"content_wrapper,omitempty"`

	// ResponseJSONPath specifies where to extract the generated text from the response.
	// Default: "choices[0].message.content" (OpenAI format)
	// Example: "content[0].text" (Anthropic format)
	ResponseJSONPath string `yaml:"response_json_path,omitempty"`

	// ExtraHeaders contains additional HTTP headers to send with each request.
	// Example: {"anthropic-version": "2023-06-01"}
	ExtraHeaders map[string]string `yaml:"extra_headers,omitempty"`
}

// PromptMessage follows the role/content pair required by most chat APIs.
type PromptMessage struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

// API Format Constants define standard values for APIFormat fields.
const (
	// Auth header defaults
	DefaultAuthHeaderName   = "Authorization"
	DefaultAuthHeaderPrefix = "Bearer "

	// System message modes
	SystemMessageModeInline   = "inline"   // Default: system messages in messages array
	SystemMessageModeSeparate = "separate" // Anthropic: system messages in separate field

	// Content wrappers
	ContentWrapperStandard  = "standard"  // Default: direct string content
	ContentWrapperAnthropic = "anthropic" // Anthropic: wrap in content array

	// Response JSON paths
	DefaultResponsePath  = "choices[0].message.content" // OpenAI/Ollama format
	AnthropicResponsePath = "content[0].text"            // Anthropic format
)

// GetAuthHeaderName returns the authentication header name with default fallback.
func (f APIFormat) GetAuthHeaderName() string {
	if f.AuthHeaderName == "" {
		return DefaultAuthHeaderName
	}
	return f.AuthHeaderName
}

// GetAuthHeaderPrefix returns the authentication header prefix with default fallback.
// Note: Empty string is a valid value (e.g., Anthropic), so we check if it was explicitly set.
func (f APIFormat) GetAuthHeaderPrefix() string {
	// If AuthHeaderName is customized but prefix is empty, it's intentional
	if f.AuthHeaderName != "" && f.AuthHeaderPrefix == "" {
		return ""
	}
	// Default OpenAI-style Bearer prefix
	if f.AuthHeaderPrefix == "" && f.AuthHeaderName == "" {
		return DefaultAuthHeaderPrefix
	}
	return f.AuthHeaderPrefix
}

// GetSystemMessageMode returns the system message handling mode with default fallback.
func (f APIFormat) GetSystemMessageMode() string {
	if f.SystemMessageMode == "" {
		return SystemMessageModeInline
	}
	return f.SystemMessageMode
}

// GetContentWrapper returns the content wrapper format with default fallback.
func (f APIFormat) GetContentWrapper() string {
	if f.ContentWrapper == "" {
		return ContentWrapperStandard
	}
	return f.ContentWrapper
}

// GetResponseJSONPath returns the JSON path for extracting response content with default fallback.
func (f APIFormat) GetResponseJSONPath() string {
	if f.ResponseJSONPath == "" {
		return DefaultResponsePath
	}
	return f.ResponseJSONPath
}

// IsSystemMessageSeparate returns true if system messages should be in a separate field.
func (f APIFormat) IsSystemMessageSeparate() bool {
	return f.GetSystemMessageMode() == SystemMessageModeSeparate
}

// IsContentWrapped returns true if content should be wrapped in Anthropic's array format.
func (f APIFormat) IsContentWrapped() bool {
	return f.GetContentWrapper() == ContentWrapperAnthropic
}
