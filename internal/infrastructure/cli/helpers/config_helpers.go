package helpers

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/doeshing/shai-go/internal/app"
	"github.com/doeshing/shai-go/internal/domain"
	configinfra "github.com/doeshing/shai-go/internal/infrastructure"
	"github.com/doeshing/shai-go/internal/services"
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

	if err := services.Validate(cfg); err != nil {
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
