package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestEnvironment creates a test workflow manager
func setupTestEnvironment(t *testing.T) (*Manager, string, func()) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "workflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create workflow manager
	manager, err := NewManager(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create workflow manager: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return manager, tmpDir, cleanup
}

func TestNewManager(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	if manager.workflowsPath == "" {
		t.Error("Expected workflowsPath to be set")
	}
}

func TestCreateWorkflow(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name:        "test-workflow",
			Description: "Test workflow",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "test-step",
							Command: "echo 'test'",
						},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(manager.workflowsPath, "test-workflow.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Workflow file was not created")
	}

	// Verify metadata was set
	if workflow.APIVersion != WorkflowAPIVersion {
		t.Errorf("Expected APIVersion %s, got %s", WorkflowAPIVersion, workflow.APIVersion)
	}

	if workflow.Kind != WorkflowKind {
		t.Errorf("Expected Kind %s, got %s", WorkflowKind, workflow.Kind)
	}

	if workflow.Metadata.Created.IsZero() {
		t.Error("Expected Created timestamp to be set")
	}
}

func TestCreateWorkflow_DuplicateName(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "duplicate-workflow",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "test-step",
							Command: "echo 'test'",
						},
					},
				},
			},
		},
	}

	// Create first workflow
	if err := manager.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create first workflow: %v", err)
	}

	// Try to create duplicate
	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error when creating duplicate workflow, got nil")
	}
}

func TestGetWorkflow(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create workflow
	original := &Workflow{
		Metadata: WorkflowMetadata{
			Name:        "get-test-workflow",
			Description: "Test description",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "test-step",
							Command: "echo 'test'",
						},
					},
				},
			},
		},
	}

	if err := manager.CreateWorkflow(original); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// Get workflow
	retrieved, err := manager.GetWorkflow("get-test-workflow")
	if err != nil {
		t.Fatalf("Failed to get workflow: %v", err)
	}

	if retrieved.Metadata.Name != original.Metadata.Name {
		t.Errorf("Expected name %s, got %s", original.Metadata.Name, retrieved.Metadata.Name)
	}

	if retrieved.Metadata.Description != original.Metadata.Description {
		t.Errorf("Expected description %s, got %s", original.Metadata.Description, retrieved.Metadata.Description)
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	_, err := manager.GetWorkflow("nonexistent-workflow")
	if err == nil {
		t.Error("Expected error when getting nonexistent workflow, got nil")
	}
}

func TestUpdateWorkflow(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create workflow
	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name:        "update-test-workflow",
			Description: "Original description",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "test-step",
							Command: "echo 'test'",
						},
					},
				},
			},
		},
	}

	if err := manager.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// Update workflow
	originalUpdated := workflow.Metadata.Updated
	time.Sleep(10 * time.Millisecond) // Ensure timestamp difference

	workflow.Metadata.Description = "Updated description"
	if err := manager.UpdateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to update workflow: %v", err)
	}

	// Verify update
	updated, err := manager.GetWorkflow("update-test-workflow")
	if err != nil {
		t.Fatalf("Failed to get updated workflow: %v", err)
	}

	if updated.Metadata.Description != "Updated description" {
		t.Errorf("Expected description 'Updated description', got %s", updated.Metadata.Description)
	}

	if !updated.Metadata.Updated.After(originalUpdated) {
		t.Error("Expected Updated timestamp to be newer")
	}
}

func TestDeleteWorkflow(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create workflow
	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "delete-test-workflow",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "test-step",
							Command: "echo 'test'",
						},
					},
				},
			},
		},
	}

	if err := manager.CreateWorkflow(workflow); err != nil {
		t.Fatalf("Failed to create workflow: %v", err)
	}

	// Delete workflow
	if err := manager.DeleteWorkflow("delete-test-workflow"); err != nil {
		t.Fatalf("Failed to delete workflow: %v", err)
	}

	// Verify deletion
	_, err := manager.GetWorkflow("delete-test-workflow")
	if err == nil {
		t.Error("Expected error when getting deleted workflow, got nil")
	}
}

func TestListWorkflows(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create multiple workflows
	workflows := []*Workflow{
		{
			Metadata: WorkflowMetadata{Name: "workflow-1"},
			Spec: WorkflowSpec{
				Jobs: []WorkflowJob{
					{
						Name: "job-1",
						Steps: []WorkflowStep{
							{Name: "step-1", Command: "echo 'test'"},
						},
					},
				},
			},
		},
		{
			Metadata: WorkflowMetadata{Name: "workflow-2"},
			Spec: WorkflowSpec{
				Jobs: []WorkflowJob{
					{
						Name: "job-1",
						Steps: []WorkflowStep{
							{Name: "step-1", Command: "echo 'test'"},
						},
					},
				},
			},
		},
		{
			Metadata: WorkflowMetadata{Name: "workflow-3"},
			Spec: WorkflowSpec{
				Jobs: []WorkflowJob{
					{
						Name: "job-1",
						Steps: []WorkflowStep{
							{Name: "step-1", Command: "echo 'test'"},
						},
					},
				},
			},
		},
	}

	for _, wf := range workflows {
		if err := manager.CreateWorkflow(wf); err != nil {
			t.Fatalf("Failed to create workflow %s: %v", wf.Metadata.Name, err)
		}
	}

	// List workflows
	list, err := manager.ListWorkflows()
	if err != nil {
		t.Fatalf("Failed to list workflows: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 workflows, got %d", len(list))
	}
}

func TestValidateWorkflow_NoName(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "", // Empty name
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{Name: "test-step", Command: "echo 'test'"},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for workflow without name, got nil")
	}
}

func TestValidateWorkflow_NoJobs(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "no-jobs-workflow",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{}, // No jobs
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for workflow without jobs, got nil")
	}
}

func TestValidateWorkflow_InvalidName(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "invalid@name!", // Invalid characters
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{Name: "test-step", Command: "echo 'test'"},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for workflow with invalid name, got nil")
	}
}

func TestValidateWorkflow_DuplicateJobNames(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "duplicate-jobs",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "duplicate-job",
					Steps: []WorkflowStep{
						{Name: "step-1", Command: "echo 'test'"},
					},
				},
				{
					Name: "duplicate-job", // Duplicate name
					Steps: []WorkflowStep{
						{Name: "step-2", Command: "echo 'test'"},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for workflow with duplicate job names, got nil")
	}
}

func TestValidateWorkflow_InvalidDependency(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "invalid-dependency",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name:      "job-1",
					DependsOn: []string{"nonexistent-job"}, // Invalid dependency
					Steps: []WorkflowStep{
						{Name: "step-1", Command: "echo 'test'"},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for workflow with invalid dependency, got nil")
	}
}

func TestValidateWorkflow_StepWithMultipleExecutionMethods(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "multiple-execution",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name:    "invalid-step",
							Command: "echo 'test'",
							Script:  "echo 'test'", // Both command and script
						},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for step with multiple execution methods, got nil")
	}
}

func TestValidateWorkflow_StepWithNoExecutionMethod(t *testing.T) {
	manager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	workflow := &Workflow{
		Metadata: WorkflowMetadata{
			Name: "no-execution",
		},
		Spec: WorkflowSpec{
			Jobs: []WorkflowJob{
				{
					Name: "test-job",
					Steps: []WorkflowStep{
						{
							Name: "invalid-step",
							// No command, script, or action
						},
					},
				},
			},
		},
	}

	err := manager.CreateWorkflow(workflow)
	if err == nil {
		t.Error("Expected error for step with no execution method, got nil")
	}
}

func TestIsValidWorkflowName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid simple name", "my-workflow", true},
		{"Valid with numbers", "workflow-123", true},
		{"Valid with underscores", "my_workflow", true},
		{"Valid uppercase", "MY-WORKFLOW", true},
		{"Invalid with spaces", "my workflow", false},
		{"Invalid with special chars", "my@workflow", false},
		{"Invalid with dots", "my.workflow", false},
		{"Empty string", "", false},
		{"Valid long name", "very-long-workflow-name-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidWorkflowName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidWorkflowName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateWorkflowTemplate(t *testing.T) {
	workflow := CreateWorkflowTemplate("test-template", "Test template description")

	if workflow == nil {
		t.Fatal("Expected workflow template to be created, got nil")
	}

	if workflow.Metadata.Name != "test-template" {
		t.Errorf("Expected name 'test-template', got %s", workflow.Metadata.Name)
	}

	if workflow.Metadata.Description != "Test template description" {
		t.Errorf("Expected description 'Test template description', got %s", workflow.Metadata.Description)
	}

	if workflow.APIVersion != WorkflowAPIVersion {
		t.Errorf("Expected APIVersion %s, got %s", WorkflowAPIVersion, workflow.APIVersion)
	}

	if workflow.Kind != WorkflowKind {
		t.Errorf("Expected Kind %s, got %s", WorkflowKind, workflow.Kind)
	}

	if len(workflow.Spec.Jobs) == 0 {
		t.Error("Expected template to have jobs")
	}
}

func TestGetWorkflowsPath(t *testing.T) {
	manager, tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	expectedPath := filepath.Join(tmpDir, WorkflowsDir)
	actualPath := manager.GetWorkflowsPath()

	if actualPath != expectedPath {
		t.Errorf("Expected workflows path %s, got %s", expectedPath, actualPath)
	}
}
