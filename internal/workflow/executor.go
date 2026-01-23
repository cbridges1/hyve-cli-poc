package workflow

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/kubeconfig"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/types"
)

// Executor handles workflow execution
type Executor struct {
	manager           *Manager
	kubeconfigManager *kubeconfig.Manager
	execution         *WorkflowExecution
	currentCluster    string
	variables         map[string]string
	workingDir        string
}

// NewExecutor creates a new workflow executor
func NewExecutor(manager *Manager, cluster string) (*Executor, error) {
	var kubeconfigMgr *kubeconfig.Manager
	var err error

	if cluster != "" {
		kubeconfigMgr, err = kubeconfig.NewManager(manager.currentRepo.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubeconfig manager: %w", err)
		}
	}

	return &Executor{
		manager:           manager,
		kubeconfigManager: kubeconfigMgr,
		currentCluster:    cluster,
		variables:         make(map[string]string),
		workingDir:        manager.currentRepo.LocalPath,
	}, nil
}

// RunWorkflow executes a workflow
func (e *Executor) RunWorkflow(ctx context.Context, workflowName string, cluster string) (*WorkflowExecution, error) {
	workflow, err := e.manager.GetWorkflow(workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	// Use specified cluster or default
	targetCluster := cluster
	if targetCluster == "" {
		targetCluster = e.currentCluster
	}

	// Create execution instance
	execution := &WorkflowExecution{
		ID:           generateExecutionID(),
		WorkflowName: workflowName,
		Cluster:      targetCluster,
		Status:       StatusRunning,
		StartTime:    time.Now(),
		Trigger:      "manual",
		JobResults:   make(map[string]*JobResult),
		Logs:         []WorkflowLogEntry{},
		Variables:    make(map[string]string),
	}

	e.execution = execution
	e.addLog("INFO", "", "", fmt.Sprintf("Starting workflow '%s'", workflowName))

	// Validate workflow requirements
	if workflow.Spec.Requirements != nil {
		e.addLog("INFO", "", "", "Validating workflow requirements...")
		validator, err := NewRequirementValidator()
		if err != nil {
			e.execution.Status = StatusFailed
			e.addLog("ERROR", "", "", fmt.Sprintf("Failed to create requirement validator: %v", err))
			return execution, fmt.Errorf("failed to create requirement validator: %w", err)
		}
		defer validator.Close()

		// Validate all requirements
		if err := validator.ValidateRequirements(workflow.Spec.Requirements); err != nil {
			e.execution.Status = StatusFailed
			e.addLog("ERROR", "", "", fmt.Sprintf("Requirements validation failed: %v", err))
			return execution, fmt.Errorf("requirements validation failed: %w", err)
		}

		// Load secrets into environment
		if err := validator.LoadSecretsIntoEnvironment(workflow.Spec.Requirements); err != nil {
			e.execution.Status = StatusFailed
			e.addLog("ERROR", "", "", fmt.Sprintf("Failed to load secrets: %v", err))
			return execution, fmt.Errorf("failed to load secrets: %w", err)
		}

		e.addLog("INFO", "", "", "✅ All requirements validated successfully")
	}

	// Set up kubeconfig if cluster specified
	var kubeconfigPath string
	if targetCluster != "" {
		kubeconfigPath, err = e.setupKubeconfig(targetCluster)
		if err != nil {
			e.execution.Status = StatusFailed
			e.addLog("ERROR", "", "", fmt.Sprintf("Failed to setup kubeconfig: %v", err))
			return execution, fmt.Errorf("failed to setup kubeconfig: %w", err)
		}
		defer e.cleanupKubeconfig(kubeconfigPath)
	}

	// Set up environment variables
	if err := e.setupEnvironmentVariables(ctx, workflow, targetCluster, kubeconfigPath); err != nil {
		e.execution.Status = StatusFailed
		e.addLog("ERROR", "", "", fmt.Sprintf("Failed to setup environment variables: %v", err))
		return execution, fmt.Errorf("failed to setup environment variables: %w", err)
	}

	// Execute jobs
	if err := e.executeJobs(ctx, workflow); err != nil {
		e.execution.Status = StatusFailed
		e.addLog("ERROR", "", "", fmt.Sprintf("Workflow failed: %v", err))
		e.finalizeExecution()
		return execution, fmt.Errorf("workflow execution failed: %w", err)
	}

	e.execution.Status = StatusCompleted
	e.addLog("INFO", "", "", "Workflow completed successfully")
	e.finalizeExecution()

	return execution, nil
}

// executeJobs executes all jobs in the workflow
func (e *Executor) executeJobs(ctx context.Context, workflow *Workflow) error {
	// Build dependency graph
	jobDeps := make(map[string][]string)
	for _, job := range workflow.Spec.Jobs {
		jobDeps[job.Name] = job.DependsOn
	}

	// Execute jobs in dependency order
	completed := make(map[string]bool)

	for len(completed) < len(workflow.Spec.Jobs) {
		progress := false

		for _, job := range workflow.Spec.Jobs {
			if completed[job.Name] {
				continue
			}

			// Check if all dependencies are completed
			canRun := true
			for _, dep := range job.DependsOn {
				if !completed[dep] {
					canRun = false
					break
				}
			}

			if !canRun {
				continue
			}

			// Execute job
			e.addLog("INFO", job.Name, "", fmt.Sprintf("Starting job '%s'", job.Name))

			jobResult, err := e.executeJob(ctx, &job, workflow)
			e.execution.JobResults[job.Name] = jobResult

			if err != nil {
				e.addLog("ERROR", job.Name, "", fmt.Sprintf("Job failed: %v", err))
				return fmt.Errorf("job '%s' failed: %w", job.Name, err)
			}

			completed[job.Name] = true
			progress = true
			e.addLog("INFO", job.Name, "", fmt.Sprintf("Job '%s' completed successfully", job.Name))
		}

		if !progress {
			return fmt.Errorf("circular dependency detected in jobs")
		}
	}

	return nil
}

// executeJob executes a single job
func (e *Executor) executeJob(ctx context.Context, job *WorkflowJob, workflow *Workflow) (*JobResult, error) {
	result := &JobResult{
		Status:    JobStatusRunning,
		StartTime: time.Now(),
		Steps:     make(map[string]*StepResult),
	}

	// Check job condition
	if job.If != "" {
		shouldRun, err := e.evaluateCondition(job.If)
		if err != nil {
			result.Status = JobStatusFailed
			result.Error = fmt.Sprintf("failed to evaluate condition: %v", err)
			return result, err
		}
		if !shouldRun {
			result.Status = JobStatusSkipped
			e.addLog("INFO", job.Name, "", "Job skipped due to condition")
			return result, nil
		}
	}

	// Set job-specific cluster if specified
	targetCluster := job.Cluster
	if targetCluster == "" {
		targetCluster = e.currentCluster
	}

	// Execute steps
	for _, step := range job.Steps {
		e.addLog("INFO", job.Name, step.Name, fmt.Sprintf("Starting step '%s'", step.Name))

		stepResult, err := e.executeStep(ctx, &step, job, workflow, targetCluster)
		result.Steps[step.Name] = stepResult

		if err != nil && !step.ContinueOnError {
			result.Status = JobStatusFailed
			result.Error = fmt.Sprintf("step '%s' failed: %v", step.Name, err)
			endTime := time.Now()
			result.EndTime = &endTime
			result.Duration = endTime.Sub(result.StartTime)
			e.addLog("ERROR", job.Name, step.Name, fmt.Sprintf("Step failed: %v", err))
			return result, err
		} else if err != nil {
			e.addLog("WARN", job.Name, step.Name, fmt.Sprintf("Step failed but continuing: %v", err))
		}

		e.addLog("INFO", job.Name, step.Name, fmt.Sprintf("Step '%s' completed", step.Name))
	}

	result.Status = JobStatusCompleted
	endTime := time.Now()
	result.EndTime = &endTime
	result.Duration = endTime.Sub(result.StartTime)

	return result, nil
}

// executeStep executes a single step
func (e *Executor) executeStep(ctx context.Context, step *WorkflowStep, job *WorkflowJob, workflow *Workflow, cluster string) (*StepResult, error) {
	result := &StepResult{
		Status:    JobStatusRunning,
		StartTime: time.Now(),
	}

	// Check step condition
	if step.If != "" {
		shouldRun, err := e.evaluateCondition(step.If)
		if err != nil {
			result.Status = JobStatusFailed
			result.Error = fmt.Sprintf("failed to evaluate condition: %v", err)
			return result, err
		}
		if !shouldRun {
			result.Status = JobStatusSkipped
			return result, nil
		}
	}

	// Determine command to execute
	var command string
	var args []string

	if step.Command != "" {
		parts := strings.Fields(step.Command)
		if len(parts) > 0 {
			command = parts[0]
			args = parts[1:]
		}
	} else if step.Script != "" {
		// Execute script via shell
		command = "sh"
		args = []string{"-c", step.Script}
	} else if step.Action != "" {
		// Execute predefined action
		return e.executeAction(ctx, step.Action, step.With, result)
	} else {
		result.Status = JobStatusFailed
		result.Error = "no command, script, or action specified"
		return result, fmt.Errorf("no command, script, or action specified")
	}

	// Set up command
	cmd := exec.CommandContext(ctx, command, args...)

	// Set working directory
	if step.WorkingDir != "" {
		cmd.Dir = filepath.Join(e.workingDir, step.WorkingDir)
	} else {
		cmd.Dir = e.workingDir
	}

	// Set environment variables
	cmd.Env = os.Environ()

	// Add workflow-level env vars
	for key, value := range workflow.Spec.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, e.expandVariables(value)))
	}

	// Add job-level env vars
	for key, value := range job.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, e.expandVariables(value)))
	}

	// Add step-level env vars
	for key, value := range step.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, e.expandVariables(value)))
	}

	// Execute command
	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	endTime := time.Now()
	result.EndTime = &endTime
	result.Duration = endTime.Sub(result.StartTime)

	// Print command output to user
	if len(output) > 0 {
		fmt.Print(string(output))
		if !strings.HasSuffix(string(output), "\n") {
			fmt.Println()
		}
	}

	if err != nil {
		result.Status = JobStatusFailed
		result.Error = err.Error()

		// Try to get exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}

		return result, err
	}

	result.Status = JobStatusCompleted
	result.ExitCode = 0
	return result, nil
}

// executeAction executes a predefined action
func (e *Executor) executeAction(ctx context.Context, action string, params map[string]string, result *StepResult) (*StepResult, error) {
	switch action {
	case "kubectl-apply":
		file := params["file"]
		if file == "" {
			result.Status = JobStatusFailed
			result.Error = "kubectl-apply action requires 'file' parameter"
			return result, fmt.Errorf("kubectl-apply action requires 'file' parameter")
		}
		cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", file)
		cmd.Dir = e.workingDir
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		result.Output = string(output)

		// Print command output to user
		if len(output) > 0 {
			fmt.Print(string(output))
			if !strings.HasSuffix(string(output), "\n") {
				fmt.Println()
			}
		}

		if err != nil {
			result.Status = JobStatusFailed
			result.Error = err.Error()
			return result, err
		}
		result.Status = JobStatusCompleted
		return result, nil

	case "kubectl-delete":
		file := params["file"]
		if file == "" {
			result.Status = JobStatusFailed
			result.Error = "kubectl-delete action requires 'file' parameter"
			return result, fmt.Errorf("kubectl-delete action requires 'file' parameter")
		}
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", file)
		cmd.Dir = e.workingDir
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		result.Output = string(output)

		// Print command output to user
		if len(output) > 0 {
			fmt.Print(string(output))
			if !strings.HasSuffix(string(output), "\n") {
				fmt.Println()
			}
		}

		if err != nil {
			result.Status = JobStatusFailed
			result.Error = err.Error()
			return result, err
		}
		result.Status = JobStatusCompleted
		return result, nil

	default:
		result.Status = JobStatusFailed
		result.Error = fmt.Sprintf("unknown action: %s", action)
		return result, fmt.Errorf("unknown action: %s", action)
	}
}

// setupKubeconfig sets up kubeconfig for the target cluster
func (e *Executor) setupKubeconfig(cluster string) (string, error) {
	if e.kubeconfigManager == nil {
		return "", fmt.Errorf("kubeconfig manager not initialized")
	}

	kc, err := e.kubeconfigManager.GetKubeconfig(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	if kc == nil {
		return "", fmt.Errorf("no kubeconfig found for cluster '%s'", cluster)
	}

	kubeconfigData, err := kc.GetConfig()
	if err != nil {
		return "", fmt.Errorf("failed to decrypt kubeconfig: %w", err)
	}

	// Create temporary kubeconfig file
	tempDir := filepath.Join(os.Getenv("HOME"), ".hyve", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	tempFile := filepath.Join(tempDir, fmt.Sprintf("kubeconfig-workflow-%s", cluster))
	if err := os.WriteFile(tempFile, []byte(kubeconfigData), 0600); err != nil {
		return "", fmt.Errorf("failed to write temporary kubeconfig: %w", err)
	}

	return tempFile, nil
}

// cleanupKubeconfig cleans up temporary kubeconfig file
func (e *Executor) cleanupKubeconfig(kubeconfigPath string) {
	if kubeconfigPath != "" {
		os.Remove(kubeconfigPath)
	}
}

// setupEnvironmentVariables sets up environment variables for the workflow
func (e *Executor) setupEnvironmentVariables(ctx context.Context, workflow *Workflow, targetCluster string, kubeconfigPath string) error {
	e.variables["WORKFLOW_NAME"] = workflow.Metadata.Name
	e.variables["WORKFLOW_CLUSTER"] = e.currentCluster
	e.variables["WORKFLOW_EXECUTION_ID"] = e.execution.ID
	e.variables["HYVE_REPOSITORY"] = e.manager.currentRepo.Name
	e.variables["HYVE_REPOSITORY_PATH"] = e.manager.currentRepo.LocalPath

	if kubeconfigPath != "" {
		e.variables["KUBECONFIG"] = kubeconfigPath
		os.Setenv("KUBECONFIG", kubeconfigPath)
	}

	// Export cluster-specific environment variables if cluster is specified
	if targetCluster != "" {
		if err := e.exportClusterEnvironmentVariables(ctx, targetCluster); err != nil {
			log.Printf("Warning: Failed to export cluster environment variables: %v", err)
			// Don't fail the workflow if we can't get cluster info
		}
	}

	return nil
}

// exportClusterEnvironmentVariables exports cluster-specific environment variables
func (e *Executor) exportClusterEnvironmentVariables(ctx context.Context, clusterName string) error {
	// Load cluster definitions from YAML files
	clusterDef, err := e.loadClusterDefinition(clusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster definition: %w", err)
	}

	// Get API key from environment
	apiKey := os.Getenv("CIVO_TOKEN")
	if apiKey == "" {
		return fmt.Errorf("CIVO_TOKEN environment variable not set")
	}

	// Create provider for this cluster
	factory := provider.NewFactory()
	prov, err := factory.CreateProvider(clusterDef.Spec.Provider, apiKey, clusterDef.Metadata.Region)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create cluster manager
	clusterMgr := cluster.NewManager(prov)

	// Get cluster information
	clusterInfo, err := clusterMgr.GetClusterInfo(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster info: %w", err)
	}

	if clusterInfo == nil {
		return fmt.Errorf("cluster info not found")
	}

	// Set HYVE_CLUSTER_* environment variables
	e.variables["HYVE_CLUSTER_NAME"] = clusterInfo.Name
	e.variables["HYVE_CLUSTER_IP_ADDRESS"] = clusterInfo.IPAddress
	e.variables["HYVE_CLUSTER_ACCESS_PORT"] = clusterInfo.AccessPort
	e.variables["HYVE_CLUSTER_ID"] = clusterInfo.ID
	e.variables["HYVE_CLUSTER_STATUS"] = clusterInfo.Status
	e.variables["HYVE_CLUSTER_KUBECONFIG"] = clusterInfo.Kubeconfig

	// Also export to process environment so they're available in scripts
	os.Setenv("HYVE_CLUSTER_NAME", clusterInfo.Name)
	os.Setenv("HYVE_CLUSTER_IP_ADDRESS", clusterInfo.IPAddress)
	os.Setenv("HYVE_CLUSTER_ACCESS_PORT", clusterInfo.AccessPort)
	os.Setenv("HYVE_CLUSTER_ID", clusterInfo.ID)
	os.Setenv("HYVE_CLUSTER_STATUS", clusterInfo.Status)
	os.Setenv("HYVE_CLUSTER_KUBECONFIG", clusterInfo.Kubeconfig)

	log.Printf("✅ Exported cluster information to environment:")
	log.Printf("  HYVE_CLUSTER_NAME=%s", clusterInfo.Name)
	log.Printf("  HYVE_CLUSTER_IP_ADDRESS=%s", clusterInfo.IPAddress)
	log.Printf("  HYVE_CLUSTER_ACCESS_PORT=%s", clusterInfo.AccessPort)
	log.Printf("  HYVE_CLUSTER_ID=%s", clusterInfo.ID)
	log.Printf("  HYVE_CLUSTER_STATUS=%s", clusterInfo.Status)

	return nil
}

// loadClusterDefinition loads a cluster definition from YAML file
func (e *Executor) loadClusterDefinition(clusterName string) (*types.ClusterDefinition, error) {
	clustersDir := filepath.Join(e.manager.currentRepo.LocalPath, "clusters")
	var clusterDef *types.ClusterDefinition

	// Check if clusters directory exists
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("clusters directory not found at %s", clustersDir)
	}

	err := filepath.WalkDir(clustersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		var cluster types.ClusterDefinition
		if err := yaml.Unmarshal(data, &cluster); err != nil {
			return fmt.Errorf("failed to unmarshal cluster definition from %s: %w", path, err)
		}

		if cluster.Metadata.Name == clusterName {
			clusterDef = &cluster
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if clusterDef == nil {
		return nil, fmt.Errorf("cluster %s not found in clusters directory", clusterName)
	}

	return clusterDef, nil
}

// evaluateCondition evaluates a condition string
func (e *Executor) evaluateCondition(condition string) (bool, error) {
	// Simple condition evaluation - can be extended
	// For now, just support basic variable checks
	expanded := e.expandVariables(condition)
	return expanded == "true", nil
}

// expandVariables expands variables in a string
func (e *Executor) expandVariables(input string) string {
	result := input
	for key, value := range e.variables {
		result = strings.ReplaceAll(result, fmt.Sprintf("${%s}", key), value)
		result = strings.ReplaceAll(result, fmt.Sprintf("$%s", key), value)
	}
	return result
}

// addLog adds a log entry to the execution
func (e *Executor) addLog(level, job, step, message string) {
	entry := WorkflowLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Job:       job,
		Step:      step,
		Message:   message,
	}

	e.execution.Logs = append(e.execution.Logs, entry)

	// Also log to stdout for immediate feedback
	prefix := fmt.Sprintf("[%s]", level)
	if job != "" {
		prefix += fmt.Sprintf("[%s]", job)
	}
	if step != "" {
		prefix += fmt.Sprintf("[%s]", step)
	}
	log.Printf("%s %s", prefix, message)
}

// finalizeExecution finalizes the workflow execution
func (e *Executor) finalizeExecution() {
	endTime := time.Now()
	e.execution.EndTime = &endTime
	e.execution.Duration = endTime.Sub(e.execution.StartTime)
}

// generateExecutionID generates a unique execution ID
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().Unix())
}

// Close closes the executor and cleans up resources
func (e *Executor) Close() error {
	if e.kubeconfigManager != nil {
		return e.kubeconfigManager.Close()
	}
	return nil
}
