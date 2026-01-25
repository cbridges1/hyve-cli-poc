package cmd

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"civo-cluster-deploy/internal/cluster"
	"civo-cluster-deploy/internal/config"
	"civo-cluster-deploy/internal/credentials"
	"civo-cluster-deploy/internal/kubeconfig"
	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/repository"
	"civo-cluster-deploy/internal/template"
	"civo-cluster-deploy/internal/types"
	"civo-cluster-deploy/internal/workflow"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage cluster templates",
	Long:  "Create, list, delete, and execute cluster templates with associated workflows",
}

var templateCreateCmd = &cobra.Command{
	Use:   "create [template-name]",
	Short: "Create a new cluster template",
	Long: `Create a new cluster template with cluster specifications and workflows.

The template will be created interactively, prompting for cluster details
and workflows to execute upon cluster creation.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		templateName := args[0]
		description, _ := cmd.Flags().GetString("description")
		provider, _ := cmd.Flags().GetString("provider")
		region, _ := cmd.Flags().GetString("region")
		nodes, _ := cmd.Flags().GetString("nodes")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		ingressEnabled, _ := cmd.Flags().GetBool("ingress")
		loadBalancer, _ := cmd.Flags().GetBool("load-balancer")
		workflows, _ := cmd.Flags().GetString("workflows")

		createTemplate(templateName, description, provider, region, nodes, clusterType, ingressEnabled, loadBalancer, workflows)
	},
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cluster templates",
	Long:  "Display all available cluster templates in the current repository",
	Run: func(cmd *cobra.Command, args []string) {
		listTemplates()
	},
}

var templateDeleteCmd = &cobra.Command{
	Use:   "delete [template-name]",
	Short: "Delete a cluster template",
	Long:  "Remove a cluster template from the repository",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		templateName := args[0]
		deleteTemplate(templateName)
	},
}

var templateExecuteCmd = &cobra.Command{
	Use:   "execute [template-name] [cluster-name]",
	Short: "Create a cluster from a template",
	Long: `Execute a cluster template to create a new cluster.

This command:
  1. Creates a cluster based on the template specifications
  2. Waits for the cluster to become ready
  3. Executes all workflows defined in the template`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		templateName := args[0]
		clusterName := args[1]
		executeTemplate(templateName, clusterName)
	},
}

var templateShowCmd = &cobra.Command{
	Use:   "show [template-name]",
	Short: "Show template details",
	Long:  "Display the full details of a cluster template",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		templateName := args[0]
		showTemplate(templateName)
	},
}

func init() {
	templateCreateCmd.Flags().StringP("description", "d", "", "Template description")
	templateCreateCmd.Flags().StringP("provider", "p", "civo", "Cloud provider")
	templateCreateCmd.Flags().StringP("region", "r", "PHX1", "Region")
	templateCreateCmd.Flags().StringP("nodes", "n", "g4s.kube.small", "Node sizes (comma-separated)")
	templateCreateCmd.Flags().StringP("cluster-type", "t", "k3s", "Kubernetes cluster type")
	templateCreateCmd.Flags().Bool("ingress", true, "Enable ingress controller")
	templateCreateCmd.Flags().Bool("load-balancer", true, "Enable load balancer for ingress")
	templateCreateCmd.Flags().StringP("workflows", "w", "", "Workflows to run after creation (comma-separated)")

	templateCmd.AddCommand(templateCreateCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateDeleteCmd)
	templateCmd.AddCommand(templateExecuteCmd)
	templateCmd.AddCommand(templateShowCmd)
}

func createTemplate(name, description, provider, region, nodesSizes, clusterType string, ingressEnabled, loadBalancer bool, workflowsStr string) {
	// Get repository path
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	// Create template manager
	templateMgr := template.NewManager(currentRepo.LocalPath)

	// Parse nodes
	nodes := strings.Split(nodesSizes, ",")
	for i, node := range nodes {
		nodes[i] = strings.TrimSpace(node)
	}

	// Parse workflows
	var workflows []string
	if workflowsStr != "" {
		workflows = strings.Split(workflowsStr, ",")
		for i, wf := range workflows {
			workflows[i] = strings.TrimSpace(wf)
		}
	}

	// Create template
	tmpl := &template.Template{
		APIVersion: "v1",
		Kind:       "Template",
		Metadata: template.TemplateMetadata{
			Name:        name,
			Description: description,
		},
		Spec: template.TemplateSpec{
			Provider:    provider,
			Region:      region,
			Nodes:       nodes,
			ClusterType: clusterType,
			Workflows:   workflows,
		},
	}

	tmpl.Spec.Ingress.Enabled = ingressEnabled
	tmpl.Spec.Ingress.LoadBalancer = loadBalancer

	// Save template
	if err := templateMgr.CreateTemplate(tmpl); err != nil {
		log.Fatalf("Failed to create template: %v", err)
	}

	log.Printf("✅ Template '%s' created successfully", name)
	log.Printf("Template path: %s", templateMgr.GetTemplatePath(name))
	log.Println("\n📋 Template Details:")
	log.Printf("  Provider: %s", provider)
	log.Printf("  Region: %s", region)
	log.Printf("  Nodes: %s", strings.Join(nodes, ", "))
	log.Printf("  Cluster Type: %s", clusterType)
	log.Printf("  Ingress: %v", ingressEnabled)
	if len(workflows) > 0 {
		log.Printf("  Workflows: %s", strings.Join(workflows, ", "))
	}

	log.Println("\n💡 Execute this template with:")
	log.Printf("  hyve template execute %s <cluster-name>", name)
}

func listTemplates() {
	// Get repository path
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	// Create template manager
	templateMgr := template.NewManager(currentRepo.LocalPath)

	templates, err := templateMgr.ListTemplates()
	if err != nil {
		log.Fatalf("Failed to list templates: %v", err)
	}

	if len(templates) == 0 {
		log.Println("No templates found")
		log.Println("\n💡 Create a template with:")
		log.Println("  hyve template create <name> --region PHX1 --nodes g4s.kube.medium")
		return
	}

	log.Printf("📋 Available templates (%d):\n", len(templates))
	for _, tmpl := range templates {
		log.Printf("  %s", tmpl.Metadata.Name)
		if tmpl.Metadata.Description != "" {
			log.Printf("    Description: %s", tmpl.Metadata.Description)
		}
		log.Printf("    Region: %s | Nodes: %s | Type: %s",
			tmpl.Spec.Region,
			strings.Join(tmpl.Spec.Nodes, ", "),
			tmpl.Spec.ClusterType)
		if len(tmpl.Spec.Workflows) > 0 {
			log.Printf("    Workflows: %s", strings.Join(tmpl.Spec.Workflows, ", "))
		}
		log.Println()
	}

	log.Println("💡 Execute a template with:")
	log.Println("  hyve template execute <template-name> <cluster-name>")
}

func deleteTemplate(name string) {
	// Get repository path
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	// Create template manager
	templateMgr := template.NewManager(currentRepo.LocalPath)

	// Delete template
	if err := templateMgr.DeleteTemplate(name); err != nil {
		log.Fatalf("Failed to delete template: %v", err)
	}

	log.Printf("✅ Template '%s' deleted successfully", name)
}

func showTemplate(name string) {
	// Get repository path
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	// Create template manager
	templateMgr := template.NewManager(currentRepo.LocalPath)

	// Get template
	tmpl, err := templateMgr.GetTemplate(name)
	if err != nil {
		log.Fatalf("Failed to get template: %v", err)
	}

	// Marshal to YAML for display
	data, err := yaml.Marshal(tmpl)
	if err != nil {
		log.Fatalf("Failed to marshal template: %v", err)
	}

	log.Printf("📋 Template: %s\n", name)
	log.Println(string(data))
}

func executeTemplate(templateName, clusterName string) {
	ctx := context.Background()

	// Get configuration
	configMgr := config.NewManager()
	apiKey := configMgr.GetCivoToken()
	if apiKey == "" {
		log.Fatal("CIVO_TOKEN environment variable is required")
	}

	// Get repository path
	repoMgr, err := repository.NewManager()
	if err != nil {
		log.Fatalf("Failed to create repository manager: %v", err)
	}
	defer repoMgr.Close()

	currentRepo, err := repoMgr.GetCurrentRepository()
	if err != nil {
		log.Println("❌ No Git repository configured")
		return
	}

	// Get authentication
	credsMgr, err := credentials.NewManager()
	var authToken string
	var authUsername = currentRepo.Username

	if err == nil {
		defer credsMgr.Close()
		if creds, _ := credsMgr.GetCredentials(); creds != nil {
			if password, err := creds.GetPassword(); err == nil && password != "" {
				authToken = password
				if authUsername == "" {
					authUsername = creds.Username
				}
			}
		}
	}

	if authToken == "" {
		authToken = os.Getenv("HYVE_GIT_TOKEN")
	}

	// Create template manager
	templateMgr := template.NewManager(currentRepo.LocalPath)

	log.Printf("🚀 Executing template '%s' to create cluster '%s'...\n", templateName, clusterName)

	// Execute template (get cluster definition)
	tmpl, clusterDef, err := templateMgr.ExecuteTemplate(ctx, templateName, clusterName)
	if err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}

	log.Println("📋 Template Details:")
	log.Printf("  Provider: %s", tmpl.Spec.Provider)
	log.Printf("  Region: %s", tmpl.Spec.Region)
	log.Printf("  Nodes: %s", strings.Join(tmpl.Spec.Nodes, ", "))
	log.Printf("  Cluster Type: %s", tmpl.Spec.ClusterType)
	if len(tmpl.Spec.Workflows) > 0 {
		log.Printf("  Workflows: %s", strings.Join(tmpl.Spec.Workflows, ", "))
	}

	// Save cluster definition to clusters directory
	clustersDir := filepath.Join(currentRepo.LocalPath, "clusters")
	if err := os.MkdirAll(clustersDir, 0755); err != nil {
		log.Fatalf("Failed to create clusters directory: %v", err)
	}

	clusterPath := filepath.Join(clustersDir, clusterName+".yaml")

	// Marshal cluster definition
	clusterData, err := yaml.Marshal(clusterDef)
	if err != nil {
		log.Fatalf("Failed to marshal cluster definition: %v", err)
	}

	if err := os.WriteFile(clusterPath, clusterData, 0644); err != nil {
		log.Fatalf("Failed to write cluster file: %v", err)
	}

	log.Printf("\n✅ Cluster definition created: %s", clusterPath)

	// Create cluster manager
	factory := provider.NewFactory()
	prov, err := factory.CreateProvider(clusterDef.Spec.Provider, apiKey, clusterDef.Metadata.Region)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	clusterMgr := cluster.NewManager(prov)

	// Create cluster
	log.Println("\n1️⃣ Creating cluster...")
	action := clusterMgr.DetermineAction(ctx, *clusterDef)

	var clusterID string
	if action == types.ActionCreate {
		createdCluster, err := clusterMgr.Create(ctx, *clusterDef)
		if err != nil {
			log.Fatalf("Failed to create cluster: %v", err)
		}
		clusterID = createdCluster.ID
		log.Printf("✅ Cluster '%s' created successfully (ID: %s)", clusterName, clusterID)
	} else {
		// Cluster already exists, get its ID
		existingCluster, err := prov.FindClusterByName(ctx, clusterName)
		if err != nil {
			log.Fatalf("Failed to find existing cluster: %v", err)
		}
		if existingCluster == nil {
			log.Fatalf("Cluster '%s' not found", clusterName)
		}
		clusterID = existingCluster.ID
		log.Printf("✅ Cluster '%s' already exists (ID: %s)", clusterName, clusterID)
	}

	// Wait for cluster to be ready
	log.Println("\n2️⃣ Waiting for cluster to be ready...")
	log.Printf("Polling cluster status (this may take several minutes)...")
	if err := clusterMgr.WaitForReady(ctx, clusterID); err != nil {
		log.Fatalf("❌ Failed to wait for cluster: %v\nCluster may not be ready. Please check the cluster status manually.", err)
	}
	log.Printf("✅ Cluster '%s' is ready", clusterName)

	// Sync kubeconfig before running workflows
	log.Println("\n3️⃣ Syncing kubeconfig...")
	clusterInfo, err := prov.GetClusterInfo(ctx, clusterName)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to get cluster info: %v", err)
		log.Println("Workflows may fail without valid kubeconfig")
	} else {
		// Save kubeconfig to database
		kubeconfigMgr, err := kubeconfig.NewManager(currentRepo.Name)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to create kubeconfig manager: %v", err)
		} else {
			defer kubeconfigMgr.Close()

			if _, err := kubeconfigMgr.StoreKubeconfig(clusterName, clusterInfo.Kubeconfig); err != nil {
				log.Printf("⚠️  Warning: Failed to store kubeconfig: %v", err)
			} else {
				log.Printf("✅ Kubeconfig synced and stored for cluster '%s'", clusterName)
			}
		}
	}

	// Execute workflows if any
	if len(tmpl.Spec.Workflows) > 0 {
		log.Printf("\n4️⃣ Executing %d workflow(s)...\n", len(tmpl.Spec.Workflows))

		// Create workflow manager
		workflowMgr, err := workflow.NewManager()
		if err != nil {
			log.Printf("⚠️  Failed to create workflow manager: %v", err)
			return
		}

		// Create executor
		executor, err := workflow.NewExecutor(workflowMgr, clusterName)
		if err != nil {
			log.Printf("⚠️  Failed to create workflow executor: %v", err)
			return
		}

		for i, workflowName := range tmpl.Spec.Workflows {
			log.Printf("\n[%d/%d] Running workflow: %s", i+1, len(tmpl.Spec.Workflows), workflowName)

			// Execute workflow
			execution, err := executor.RunWorkflow(ctx, workflowName, clusterName)
			if err != nil {
				log.Printf("❌ Workflow '%s' failed: %v", workflowName, err)
				continue
			}

			if execution.Status == workflow.StatusCompleted {
				log.Printf("✅ Workflow '%s' completed successfully", workflowName)
			} else {
				log.Printf("❌ Workflow '%s' failed", workflowName)
			}
		}
	}

	log.Printf("\n✅ Template execution completed!")
	log.Printf("\n💡 Cluster '%s' is now available", clusterName)
	log.Println("💡 Use 'hyve kubeconfig sync' to get the kubeconfig")
}
