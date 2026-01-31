package workflow

import (
	"os"
	"testing"
)

func TestExtractVersion(t *testing.T) {
	validator := &RequirementValidator{}

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "kubectl version",
			output:   "kubectl version v1.28.0",
			expected: "1.28.0",
		},
		{
			name:     "helm version",
			output:   "version.BuildInfo{Version:\"v3.12.0\", GitCommit:\"\", GitTreeState:\"\"}",
			expected: "3.12.0",
		},
		{
			name:     "docker version",
			output:   "Docker version 20.10.21, build baeda1f",
			expected: "20.10.21",
		},
		{
			name:     "version without v prefix",
			output:   "Version: 2.8.1",
			expected: "2.8.1",
		},
		{
			name:     "no version found",
			output:   "Some random output",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.extractVersion(tt.output)
			if result != tt.expected {
				t.Errorf("extractVersion(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

func TestVersionSatisfies(t *testing.T) {
	validator := &RequirementValidator{}

	tests := []struct {
		name     string
		actual   string
		required string
		expected bool
	}{
		{
			name:     "exact match",
			actual:   "1.28.0",
			required: "1.28.0",
			expected: true,
		},
		{
			name:     "newer version",
			actual:   "1.29.0",
			required: "1.28.0",
			expected: true,
		},
		{
			name:     "older version",
			actual:   "1.27.0",
			required: "1.28.0",
			expected: false,
		},
		{
			name:     "with v prefix",
			actual:   "v1.28.0",
			required: "v1.28.0",
			expected: true,
		},
		{
			name:     "mixed v prefix",
			actual:   "v1.28.0",
			required: "1.28.0",
			expected: true,
		},
		{
			name:     "major version newer",
			actual:   "2.0.0",
			required: "1.28.0",
			expected: true,
		},
		{
			name:     "minor version newer",
			actual:   "1.29.0",
			required: "1.28.5",
			expected: true,
		},
		{
			name:     "patch version newer",
			actual:   "1.28.5",
			required: "1.28.0",
			expected: true,
		},
		{
			name:     "two-part version",
			actual:   "3.12",
			required: "3.12",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.versionSatisfies(tt.actual, tt.required)
			if result != tt.expected {
				t.Errorf("versionSatisfies(%q, %q) = %v, want %v", tt.actual, tt.required, result, tt.expected)
			}
		})
	}
}

func TestValidateSecret_EnvironmentVariable(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	// Set environment variable
	secretName := "TEST_SECRET_VAR"
	os.Setenv(secretName, "test-value")
	defer os.Unsetenv(secretName)

	secret := SecretRequirement{
		Name:     secretName,
		Required: true,
	}

	err = validator.validateSecret(secret)
	if err != nil {
		t.Errorf("Expected no error for secret in environment, got: %v", err)
	}
}

func TestValidateSecret_NotFoundRequired(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	secret := SecretRequirement{
		Name:        "NONEXISTENT_SECRET",
		Provider:    "nonexistent-provider",
		Required:    true,
		Description: "Test secret",
	}

	err = validator.validateSecret(secret)
	if err == nil {
		t.Error("Expected error for required secret not found, got nil")
	}
}

func TestValidateSecret_NotFoundOptional(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	secret := SecretRequirement{
		Name:        "OPTIONAL_SECRET",
		Provider:    "optional-provider",
		Required:    false,
		Description: "Optional secret",
	}

	err = validator.validateSecret(secret)
	if err != nil {
		t.Errorf("Expected no error for optional secret not found, got: %v", err)
	}
}

func TestValidateRequirements_NoRequirements(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	err = validator.ValidateRequirements(nil)
	if err != nil {
		t.Errorf("Expected no error for nil requirements, got: %v", err)
	}

	err = validator.ValidateRequirements(&WorkflowRequirements{})
	if err != nil {
		t.Errorf("Expected no error for empty requirements, got: %v", err)
	}
}

func TestValidateRequirements_ToolNotFound(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	requirements := &WorkflowRequirements{
		Tools: []ToolRequirement{
			{
				Name:        "nonexistent-tool-xyz123",
				Description: "This tool does not exist",
			},
		},
	}

	err = validator.ValidateRequirements(requirements)
	if err == nil {
		t.Error("Expected error for nonexistent tool, got nil")
	}
}

func TestValidateRequirements_ToolFound(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	// Use a tool that should be available on most systems
	requirements := &WorkflowRequirements{
		Tools: []ToolRequirement{
			{
				Name:        "go",
				Description: "Go programming language",
			},
		},
	}

	err = validator.ValidateRequirements(requirements)
	if err != nil {
		t.Errorf("Expected no error for 'go' tool (should be available), got: %v", err)
	}
}

func TestLoadSecretsIntoEnvironment(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	// Set one secret in environment
	existingSecret := "EXISTING_SECRET"
	os.Setenv(existingSecret, "existing-value")
	defer os.Unsetenv(existingSecret)

	requirements := &WorkflowRequirements{
		Secrets: []SecretRequirement{
			{
				Name:     existingSecret,
				Required: false,
			},
		},
	}

	err = validator.LoadSecretsIntoEnvironment(requirements)
	if err != nil {
		t.Errorf("Expected no error loading secrets, got: %v", err)
	}

	// Verify existing secret is still there
	if os.Getenv(existingSecret) != "existing-value" {
		t.Error("Expected existing secret to remain unchanged")
	}
}

func TestLoadSecretsIntoEnvironment_NoRequirements(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	err = validator.LoadSecretsIntoEnvironment(nil)
	if err != nil {
		t.Errorf("Expected no error for nil requirements, got: %v", err)
	}

	err = validator.LoadSecretsIntoEnvironment(&WorkflowRequirements{})
	if err != nil {
		t.Errorf("Expected no error for empty requirements, got: %v", err)
	}
}

func TestNewRequirementValidator(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	if validator.credsMgr == nil {
		t.Error("Expected credentials manager to be initialized")
	}
}

func TestRequirementValidator_Close(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	err = validator.Close()
	if err != nil {
		t.Errorf("Expected no error closing validator, got: %v", err)
	}

	// Test closing with nil credsMgr
	validator2 := &RequirementValidator{credsMgr: nil}
	err = validator2.Close()
	if err != nil {
		t.Errorf("Expected no error closing validator with nil credsMgr, got: %v", err)
	}
}

func TestValidateRequirements_MultipleErrors(t *testing.T) {
	validator, err := NewRequirementValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	requirements := &WorkflowRequirements{
		Tools: []ToolRequirement{
			{
				Name:        "nonexistent-tool-1",
				Description: "First missing tool",
			},
			{
				Name:        "nonexistent-tool-2",
				Description: "Second missing tool",
			},
		},
		Secrets: []SecretRequirement{
			{
				Name:        "MISSING_SECRET_1",
				Provider:    "provider1",
				Required:    true,
				Description: "First missing secret",
			},
		},
	}

	err = validator.ValidateRequirements(requirements)
	if err == nil {
		t.Error("Expected error for multiple validation failures, got nil")
	}

	// Check that error message contains information about all failures
	errMsg := err.Error()
	if !contains(errMsg, "nonexistent-tool-1") {
		t.Error("Expected error message to mention first missing tool")
	}
	if !contains(errMsg, "nonexistent-tool-2") {
		t.Error("Expected error message to mention second missing tool")
	}
	if !contains(errMsg, "MISSING_SECRET_1") {
		t.Error("Expected error message to mention missing secret")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
