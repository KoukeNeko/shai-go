package domain

import "fmt"

// Rich Domain Model: 將業務邏輯封裝在 Domain 實體中
// 符合 Clean Code 原則 - 貧血模型 → 富領域模型

// GetDefaultModel retrieves the default model definition from configuration
// Returns an error if the default model is not found
func (c *Config) GetDefaultModel() (ModelDefinition, error) {
	if c.Preferences.DefaultModel == "" {
		return ModelDefinition{}, fmt.Errorf("no default model configured")
	}

	for _, model := range c.Models {
		if model.Name == c.Preferences.DefaultModel {
			return model, nil
		}
	}

	return ModelDefinition{}, fmt.Errorf("default model %s not found in configuration", c.Preferences.DefaultModel)
}

// FindModelByName searches for a model by its name
// Returns the model definition and true if found, empty model and false otherwise
func (c *Config) FindModelByName(name string) (ModelDefinition, bool) {
	for _, model := range c.Models {
		if model.Name == name {
			return model, true
		}
	}
	return ModelDefinition{}, false
}

// HasModel checks if a model with the given name exists in the configuration
func (c *Config) HasModel(name string) bool {
	_, exists := c.FindModelByName(name)
	return exists
}

// AddModel adds a new model to the configuration
// Returns an error if a model with the same name already exists
func (c *Config) AddModel(model ModelDefinition) error {
	if c.HasModel(model.Name) {
		return fmt.Errorf("model with name %s already exists", model.Name)
	}

	c.Models = append(c.Models, model)
	return nil
}

// RemoveModel removes a model from the configuration by name
// Returns an error if the model is not found
// Automatically updates the default model and fallback list if necessary
func (c *Config) RemoveModel(name string) error {
	indexToRemove := -1
	for i, model := range c.Models {
		if model.Name == name {
			indexToRemove = i
			break
		}
	}

	if indexToRemove == -1 {
		return fmt.Errorf("model %s not found", name)
	}

	// Remove the model
	c.Models = append(c.Models[:indexToRemove], c.Models[indexToRemove+1:]...)

	// Update default model if it was removed
	if c.Preferences.DefaultModel == name {
		c.updateDefaultModelAfterRemoval()
	}

	// Remove from fallback list
	c.removeFromFallbackModels(name)

	return nil
}

// updateDefaultModelAfterRemoval selects a new default model after the current one is removed
func (c *Config) updateDefaultModelAfterRemoval() {
	if len(c.Models) > 0 {
		c.Preferences.DefaultModel = c.Models[0].Name
	} else {
		c.Preferences.DefaultModel = ""
	}
}

// removeFromFallbackModels removes a model name from the fallback list
func (c *Config) removeFromFallbackModels(name string) {
	var updatedFallbacks []string
	for _, fallback := range c.Preferences.FallbackModels {
		if fallback != name {
			updatedFallbacks = append(updatedFallbacks, fallback)
		}
	}
	c.Preferences.FallbackModels = updatedFallbacks
}

// SetDefaultModel changes the default model to the specified name
// Returns an error if the model doesn't exist
func (c *Config) SetDefaultModel(name string) error {
	if !c.HasModel(name) {
		return fmt.Errorf("cannot set default model: model %s does not exist", name)
	}

	c.Preferences.DefaultModel = name
	return nil
}

// GetFallbackModels returns the list of fallback models that actually exist
// Filters out any fallback models that are not in the configuration
func (c *Config) GetFallbackModels() []ModelDefinition {
	var fallbackModels []ModelDefinition

	for _, fallbackName := range c.Preferences.FallbackModels {
		if model, exists := c.FindModelByName(fallbackName); exists {
			fallbackModels = append(fallbackModels, model)
		}
	}

	return fallbackModels
}

// GetModelCount returns the total number of configured models
func (c *Config) GetModelCount() int {
	return len(c.Models)
}

// IsSecurityEnabled checks if security guardrails are enabled
func (c *Config) IsSecurityEnabled() bool {
	return c.Security.Enabled
}

// ShouldConfirmBeforeExecution checks if user confirmation is required before execution
func (c *Config) ShouldConfirmBeforeExecution() bool {
	return c.Execution.ConfirmBeforeExecute
}

// ShouldAutoExecuteSafe checks if safe commands should be auto-executed
func (c *Config) ShouldAutoExecuteSafe() bool {
	return c.Preferences.AutoExecuteSafe
}

// GetExecutionShell returns the configured shell for command execution
// Returns the default shell if not configured
func (c *Config) GetExecutionShell() string {
	const defaultShell = "sh"

	if c.Execution.Shell == "" {
		return defaultShell
	}
	return c.Execution.Shell
}

// IsGitContextEnabled checks if git context collection is enabled
func (c *Config) IsGitContextEnabled() bool {
	// "auto" and "always" both mean enabled
	return c.Context.IncludeGit == "auto" || c.Context.IncludeGit == "always"
}

// IsKubernetesContextEnabled checks if Kubernetes context collection is enabled
func (c *Config) IsKubernetesContextEnabled() bool {
	return c.Context.IncludeK8s == "auto" || c.Context.IncludeK8s == "always"
}

// IsEnvironmentContextEnabled checks if environment variables should be included in context
func (c *Config) IsEnvironmentContextEnabled() bool {
	return c.Context.IncludeEnv
}

// GetMaxContextFiles returns the maximum number of files to include in context
func (c *Config) GetMaxContextFiles() int {
	const defaultMaxFiles = 5

	if c.Context.MaxFiles <= 0 {
		return defaultMaxFiles
	}
	return c.Context.MaxFiles
}

// GetCacheMaxEntries returns the maximum number of cache entries
func (c *Config) GetCacheMaxEntries() int {
	const defaultMaxEntries = 100

	if c.Cache.MaxEntries <= 0 {
		return defaultMaxEntries
	}
	return c.Cache.MaxEntries
}

// GetHistoryRetentionDays returns the number of days to retain history
func (c *Config) GetHistoryRetentionDays() int {
	const defaultRetentionDays = 30

	if c.History.RetentionDays <= 0 {
		return defaultRetentionDays
	}
	return c.History.RetentionDays
}

// GetTimeoutSeconds returns the command execution timeout in seconds
func (c *Config) GetTimeoutSeconds() int {
	const defaultTimeoutSeconds = 30

	if c.Preferences.TimeoutSeconds <= 0 {
		return defaultTimeoutSeconds
	}
	return c.Preferences.TimeoutSeconds
}

// ValidateConsistency checks the internal consistency of the configuration
// Returns an error if there are inconsistencies (e.g., default model doesn't exist)
func (c *Config) ValidateConsistency() error {
	// Check if default model exists
	if c.Preferences.DefaultModel != "" && !c.HasModel(c.Preferences.DefaultModel) {
		return fmt.Errorf("default model %s does not exist in models list", c.Preferences.DefaultModel)
	}

	// Check if fallback models exist
	for _, fallbackName := range c.Preferences.FallbackModels {
		if !c.HasModel(fallbackName) {
			return fmt.Errorf("fallback model %s does not exist in models list", fallbackName)
		}
	}

	// Validate at least one model is configured if default model is set
	if c.Preferences.DefaultModel != "" && len(c.Models) == 0 {
		return fmt.Errorf("default model is set but no models are configured")
	}

	return nil
}
