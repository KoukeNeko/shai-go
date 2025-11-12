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

type ollamaProvider struct {
	model      domain.ModelDefinition
	httpClient *http.Client
}

func newOllamaProvider(model domain.ModelDefinition, client *http.Client) ports.Provider {
	return &ollamaProvider{
		model:      model,
		httpClient: client,
	}
}

func (o *ollamaProvider) Name() string {
	return "ollama"
}

func (o *ollamaProvider) Model() domain.ModelDefinition {
	return o.model
}

func (o *ollamaProvider) Generate(ctx context.Context, req ports.ProviderRequest) (ports.ProviderResponse, error) {
	rendered, err := renderPromptMessages(o.model, req.Prompt, req.Context)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	payload := chatCompletionRequest{
		Model:     valueOrDefault(o.model.ModelID, "codellama:7b"),
		MaxTokens: valueOrDefaultInt(o.model.MaxTokens, 512),
		Messages:  toChatMessages(rendered),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ports.ProviderResponse{}, err
	}

	endpoint := valueOrDefault(o.model.Endpoint, "http://localhost:11434/v1/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	httpReq.Header.Set("content-type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return ports.ProviderResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ports.ProviderResponse{}, fmt.Errorf("ollama: %s", resp.Status)
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ports.ProviderResponse{}, err
	}
	content := decoded.FirstMessage()
	command := extractCommand(content)
	result := ports.ProviderResponse{
		Command:   command,
		Reply:     content,
		Reasoning: "Generated via Ollama",
	}
	emitStream(req, content)
	return result, nil
}
