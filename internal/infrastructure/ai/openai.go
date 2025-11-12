package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/doeshing/shai-go/internal/domain"
	"github.com/doeshing/shai-go/internal/ports"
)

type openAIProvider struct {
	model      domain.ModelDefinition
	httpClient *http.Client
}

func newOpenAIProvider(model domain.ModelDefinition, client *http.Client) ports.Provider {
	return &openAIProvider{
		model:      model,
		httpClient: client,
	}
}

func (p *openAIProvider) Name() string {
	return "openai"
}

func (p *openAIProvider) Model() domain.ModelDefinition {
	return p.model
}

func (p *openAIProvider) Generate(ctx context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	apiKey := resolveAuth(p.model.AuthEnvVar, "OPENAI_API_KEY")
	if apiKey == "" {
		return newHeuristicProvider(p.model).Generate(ctx, req)
	}

	rendered, err := renderPromptMessages(p.model, req.Prompt, req.Context)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	payload := chatCompletionRequest{
		Model:     valueOrDefault(p.model.ModelID, "gpt-4o-mini"),
		MaxTokens: valueOrDefaultInt(p.model.MaxTokens, 512),
		Messages:  toChatMessages(rendered),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	endpoint := valueOrDefault(p.model.Endpoint, "https://api.openai.com/v1/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	httpReq.Header.Set("authorization", "Bearer "+apiKey)
	httpReq.Header.Set("content-type", "application/json")
	if org := resolveOrg(p.model.OrgEnvVar, "OPENAI_ORG_ID"); org != "" {
		httpReq.Header.Set("OpenAI-Organization", org)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ports.ProviderResponse{}, fmt.Errorf("openai: %s", resp.Status)
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ports.ProviderResponse{}, err
	}
	content := decoded.FirstMessage()
	command := extractCommand(content)
	return ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: "Generated via OpenAI",
	}, nil
}
