package domain_test

import (
	"testing"

	"github.com/doeshing/shai-go/internal/domain"
)

// TestConfig_GetDefaultModel tests retrieving the default model
func TestConfig_GetDefaultModel(t *testing.T) {
	tests := []struct {
		name        string
		config      domain.Config
		wantError   bool
		wantModelID string
	}{
		{
			name: "returns default model successfully",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "claude",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude", ModelID: "claude-3-5-sonnet"},
					{Name: "gpt4", ModelID: "gpt-4o"},
				},
			},
			wantError:   false,
			wantModelID: "claude-3-5-sonnet",
		},
		{
			name: "returns error when default model not found",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "nonexistent",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude", ModelID: "claude-3-5-sonnet"},
				},
			},
			wantError: true,
		},
		{
			name: "returns error when no default model configured",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude", ModelID: "claude-3-5-sonnet"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := tt.config.GetDefaultModel()

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if model.ModelID != tt.wantModelID {
				t.Errorf("got model ID %s, want %s", model.ModelID, tt.wantModelID)
			}
		})
	}
}

// TestConfig_AddModel tests adding a new model
func TestConfig_AddModel(t *testing.T) {
	tests := []struct {
		name      string
		config    domain.Config
		newModel  domain.ModelDefinition
		wantError bool
	}{
		{
			name: "successfully adds new model",
			config: domain.Config{
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			newModel: domain.ModelDefinition{
				Name:    "gpt4",
				ModelID: "gpt-4o",
			},
			wantError: false,
		},
		{
			name: "returns error when model already exists",
			config: domain.Config{
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			newModel: domain.ModelDefinition{
				Name:    "claude",
				ModelID: "claude-3-5-sonnet",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.AddModel(tt.newModel)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !tt.config.HasModel(tt.newModel.Name) {
				t.Errorf("model %s was not added", tt.newModel.Name)
			}
		})
	}
}

// TestConfig_RemoveModel tests removing a model
func TestConfig_RemoveModel(t *testing.T) {
	tests := []struct {
		name                string
		config              domain.Config
		modelToRemove       string
		wantError           bool
		expectedDefaultName string
	}{
		{
			name: "successfully removes model",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "claude",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude"},
					{Name: "gpt4"},
				},
			},
			modelToRemove:       "gpt4",
			wantError:           false,
			expectedDefaultName: "claude",
		},
		{
			name: "updates default when removing default model",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "claude",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude"},
					{Name: "gpt4"},
				},
			},
			modelToRemove:       "claude",
			wantError:           false,
			expectedDefaultName: "gpt4",
		},
		{
			name: "returns error when model not found",
			config: domain.Config{
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			modelToRemove: "nonexistent",
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.RemoveModel(tt.modelToRemove)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.config.HasModel(tt.modelToRemove) {
				t.Errorf("model %s was not removed", tt.modelToRemove)
			}

			if tt.expectedDefaultName != "" && tt.config.Preferences.DefaultModel != tt.expectedDefaultName {
				t.Errorf("expected default model %s, got %s", tt.expectedDefaultName, tt.config.Preferences.DefaultModel)
			}
		})
	}
}

// TestConfig_SetDefaultModel tests setting the default model
func TestConfig_SetDefaultModel(t *testing.T) {
	tests := []struct {
		name      string
		config    domain.Config
		modelName string
		wantError bool
	}{
		{
			name: "successfully sets default model",
			config: domain.Config{
				Models: []domain.ModelDefinition{
					{Name: "claude"},
					{Name: "gpt4"},
				},
			},
			modelName: "gpt4",
			wantError: false,
		},
		{
			name: "returns error when model doesn't exist",
			config: domain.Config{
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			modelName: "nonexistent",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.SetDefaultModel(tt.modelName)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.config.Preferences.DefaultModel != tt.modelName {
				t.Errorf("expected default model %s, got %s", tt.modelName, tt.config.Preferences.DefaultModel)
			}
		})
	}
}

// TestConfig_ValidateConsistency tests configuration consistency validation
func TestConfig_ValidateConsistency(t *testing.T) {
	tests := []struct {
		name      string
		config    domain.Config
		wantError bool
	}{
		{
			name: "valid configuration",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel:   "claude",
					FallbackModels: []string{"gpt4"},
				},
				Models: []domain.ModelDefinition{
					{Name: "claude"},
					{Name: "gpt4"},
				},
			},
			wantError: false,
		},
		{
			name: "invalid: default model doesn't exist",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "nonexistent",
				},
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			wantError: true,
		},
		{
			name: "invalid: fallback model doesn't exist",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel:   "claude",
					FallbackModels: []string{"nonexistent"},
				},
				Models: []domain.ModelDefinition{
					{Name: "claude"},
				},
			},
			wantError: true,
		},
		{
			name: "invalid: default model set but no models configured",
			config: domain.Config{
				Preferences: domain.Preferences{
					DefaultModel: "claude",
				},
				Models: []domain.ModelDefinition{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateConsistency()

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestConfig_GetFallbackModels tests retrieving fallback models
func TestConfig_GetFallbackModels(t *testing.T) {
	config := domain.Config{
		Preferences: domain.Preferences{
			FallbackModels: []string{"gpt4", "nonexistent", "ollama"},
		},
		Models: []domain.ModelDefinition{
			{Name: "claude"},
			{Name: "gpt4"},
			{Name: "ollama"},
		},
	}

	fallbacks := config.GetFallbackModels()

	if len(fallbacks) != 2 {
		t.Errorf("expected 2 fallback models, got %d", len(fallbacks))
	}

	expectedNames := map[string]bool{"gpt4": true, "ollama": true}
	for _, model := range fallbacks {
		if !expectedNames[model.Name] {
			t.Errorf("unexpected fallback model: %s", model.Name)
		}
	}
}

// TestConfig_ContextSettings tests context-related methods
func TestConfig_ContextSettings(t *testing.T) {
	tests := []struct {
		name     string
		config   domain.Config
		checkGit bool
		checkK8s bool
		checkEnv bool
	}{
		{
			name: "git context enabled with auto",
			config: domain.Config{
				Context: domain.ContextSettings{
					IncludeGit: "auto",
				},
			},
			checkGit: true,
		},
		{
			name: "git context enabled with always",
			config: domain.Config{
				Context: domain.ContextSettings{
					IncludeGit: "always",
				},
			},
			checkGit: true,
		},
		{
			name: "git context disabled with never",
			config: domain.Config{
				Context: domain.ContextSettings{
					IncludeGit: "never",
				},
			},
			checkGit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.checkGit && !tt.config.IsGitContextEnabled() {
				t.Error("expected git context to be enabled")
			}
			if !tt.checkGit && tt.config.IsGitContextEnabled() {
				t.Error("expected git context to be disabled")
			}
		})
	}
}
