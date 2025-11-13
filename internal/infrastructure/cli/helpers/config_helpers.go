package helpers

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	configapp "github.com/doeshing/shai-go/internal/application/config"
	"github.com/doeshing/shai-go/internal/domain"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure/config"
)

// GetConfigLoader extracts the config loader from container with error handling
func GetConfigLoader(container *app.Container) (*configinfra.FileLoader, error) {
	if container.ConfigLoader == nil {
		return nil, fmt.Errorf("config loader unavailable")
	}
	return container.ConfigLoader, nil
}

// SaveConfigWithValidation validates and saves configuration with automatic backup
func SaveConfigWithValidation(container *app.Container, cfg domain.Config) error {
	loader, err := GetConfigLoader(container)
	if err != nil {
		return err
	}

	if err := configapp.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if err := createBackupIfExists(loader); err != nil {
		return err
	}

	if err := loader.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// createBackupIfExists creates a backup of the config file if it exists
func createBackupIfExists(loader *configinfra.FileLoader) error {
	if _, err := os.Stat(loader.Path()); err == nil {
		if _, err := loader.Backup(); err != nil {
			return fmt.Errorf("failed to create configuration backup: %w", err)
		}
	}
	return nil
}

// ParseYAMLValue parses a string value as YAML, falling back to literal string
func ParseYAMLValue(input string) (interface{}, error) {
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(input), &parsed); err != nil {
		// If YAML parsing fails, treat as literal string
		return input, nil
	}
	return parsed, nil
}

// SetNestedMapValue sets a value in a nested map using a key path
// Returns true if successful, false otherwise
func SetNestedMapValue(root map[string]interface{}, keyPath []string, value interface{}) bool {
	if len(keyPath) == 0 {
		return false
	}

	current := root
	for i := 0; i < len(keyPath)-1; i++ {
		key := keyPath[i]
		next, exists := current[key]

		if !exists {
			newChild := map[string]interface{}{}
			current[key] = newChild
			current = newChild
			continue
		}

		child, isMap := next.(map[string]interface{})
		if !isMap {
			// Overwrite non-map value with new map
			child = map[string]interface{}{}
			current[key] = child
		}
		current = child
	}

	current[keyPath[len(keyPath)-1]] = value
	return true
}

// TraverseNestedMap retrieves a value from a nested map using a key path
// Returns the value and true if found, nil and false otherwise
func TraverseNestedMap(data interface{}, keyPath []string) (interface{}, bool) {
	if len(keyPath) == 0 {
		return data, true
	}

	switch node := data.(type) {
	case map[string]interface{}:
		next, exists := node[keyPath[0]]
		if !exists {
			return nil, false
		}
		return TraverseNestedMap(next, keyPath[1:])
	default:
		return nil, false
	}
}

// FindModelByName searches for a model in the configuration
func FindModelByName(cfg domain.Config, name string) (domain.ModelDefinition, bool) {
	for _, model := range cfg.Models {
		if model.Name == name {
			return model, true
		}
	}
	return domain.ModelDefinition{}, false
}

// LoadPromptMessagesFromFile reads and parses a YAML file containing prompt messages
func LoadPromptMessagesFromFile(filepath string) ([]domain.PromptMessage, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file %s: %w", filepath, err)
	}

	var prompts []domain.PromptMessage
	if err := yaml.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("failed to parse prompt file %s: %w", filepath, err)
	}

	return prompts, nil
}

// RemoveModelFromList removes a model from the models list by name
// Returns the updated list and the index where it was found (-1 if not found)
func RemoveModelFromList(models []domain.ModelDefinition, name string) ([]domain.ModelDefinition, int) {
	for i, model := range models {
		if model.Name == name {
			return append(models[:i], models[i+1:]...), i
		}
	}
	return models, -1
}

// RemoveFromStringSlice removes all occurrences of a string from a slice
func RemoveFromStringSlice(slice []string, value string) []string {
	var result []string
	for _, item := range slice {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

// SplitAndTrimCSV splits a comma-separated string and trims whitespace
func SplitAndTrimCSV(input string) []string {
	if input == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
