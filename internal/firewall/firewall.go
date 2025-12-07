package firewall

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/civo/civogo"

	"civo-cluster-deploy/internal/types"
)

// Manager handles firewall operations
type Manager struct {
	client *civogo.Client
}

// NewManager creates a new firewall manager
func NewManager(client *civogo.Client) *Manager {
	return &Manager{
		client: client,
	}
}

// CreateFromDefinition creates or finds an existing firewall for a cluster
func (m *Manager) CreateFromDefinition(ctx context.Context, clusterDef types.ClusterDefinition) (*civogo.FirewallResult, error) {
	firewallName := fmt.Sprintf("%s-firewall", clusterDef.Metadata.Name)

	// First, check if firewall already exists
	existingFirewall, err := m.FindByName(firewallName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing firewall: %w", err)
	}

	if existingFirewall != nil {
		log.Printf("Firewall %s already exists with ID %s, reusing it", firewallName, existingFirewall.ID)
		return existingFirewall, nil
	}

	// Firewall doesn't exist, create a new one
	log.Printf("Creating new firewall %s", firewallName)

	networks, err := m.client.ListNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var defaultNetworkID string
	for _, network := range networks {
		if network.Default {
			defaultNetworkID = network.ID
			break
		}
	}

	if defaultNetworkID == "" {
		if len(networks) > 0 {
			defaultNetworkID = networks[0].ID
		} else {
			return nil, fmt.Errorf("no networks found in region %s", clusterDef.Metadata.Region)
		}
	}

	var rules []civogo.FirewallRule
	for _, rule := range clusterDef.Spec.Firewall.Rules {
		rules = append(rules, civogo.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	firewallConfig := &civogo.FirewallConfig{
		Name:      firewallName,
		NetworkID: defaultNetworkID,
		Region:    clusterDef.Metadata.Region,
		Rules:     rules,
	}

	firewall, err := m.client.NewFirewall(firewallConfig)
	if err != nil {
		// If creation fails, it might be because it already exists (race condition)
		// Try to find it again before failing
		existingFirewall, findErr := m.FindByName(firewallName)
		if findErr == nil && existingFirewall != nil {
			log.Printf("Firewall %s was created by another process, reusing it", firewallName)
			return existingFirewall, nil
		}
		return nil, fmt.Errorf("failed to create firewall: %w", err)
	}

	log.Printf("Successfully created firewall %s with ID %s", firewallName, firewall.ID)
	return firewall, nil
}

// FindByName finds a firewall by name
func (m *Manager) FindByName(name string) (*civogo.FirewallResult, error) {
	firewalls, err := m.client.ListFirewalls()
	if err != nil {
		return nil, err
	}

	for _, firewall := range firewalls {
		if firewall.Name == name {
			// Convert Firewall to FirewallResult format
			return &civogo.FirewallResult{
				ID:   firewall.ID,
				Name: firewall.Name,
			}, nil
		}
	}

	return nil, nil // Not found, but no error
}

// Delete deletes a firewall by ID
func (m *Manager) Delete(ctx context.Context, firewallID string) error {
	_, err := m.client.DeleteFirewall(firewallID)
	return err
}

// DeleteByName deletes a firewall by name
func (m *Manager) DeleteByName(ctx context.Context, firewallName string) error {
	firewall, err := m.FindByName(firewallName)
	if err != nil {
		return err
	}

	if firewall == nil {
		return nil // Already deleted or doesn't exist
	}

	return m.Delete(ctx, firewall.ID)
}

// FindOrphaned finds firewalls that exist but are not needed by current cluster definitions
func (m *Manager) FindOrphaned(desiredClusters []types.ClusterDefinition) ([]*civogo.FirewallResult, error) {
	// Get all existing firewalls
	allFirewalls, err := m.client.ListFirewalls()
	if err != nil {
		return nil, fmt.Errorf("failed to list firewalls: %w", err)
	}

	// Create a map of expected firewall names for desired clusters
	expectedFirewallNames := make(map[string]bool)
	for _, cluster := range desiredClusters {
		if cluster.Spec.Firewall.Enabled {
			firewallName := fmt.Sprintf("%s-firewall", cluster.Metadata.Name)
			expectedFirewallNames[firewallName] = true
		}
	}

	// Find firewalls that match our naming pattern but aren't needed
	var orphaned []*civogo.FirewallResult
	for _, firewall := range allFirewalls {
		// Check if this firewall follows our naming pattern
		if m.ShouldManage(firewall.Name) && !expectedFirewallNames[firewall.Name] {
			// Convert to FirewallResult for consistency
			orphaned = append(orphaned, &civogo.FirewallResult{
				ID:   firewall.ID,
				Name: firewall.Name,
			})
		}
	}

	return orphaned, nil
}

// ShouldManage determines if a firewall should be managed by this tool
func (m *Manager) ShouldManage(firewallName string) bool {
	// Check if this firewall should be managed by our tool
	// Look for our naming pattern: "*-firewall"
	if strings.HasSuffix(firewallName, "-firewall") {
		return true
	}

	// Add other patterns as needed
	return false
}

// CleanupOrphaned deletes orphaned firewalls
func (m *Manager) CleanupOrphaned(ctx context.Context, orphanedFirewalls []*civogo.FirewallResult) error {
	if len(orphanedFirewalls) == 0 {
		log.Println("No orphaned firewalls found")
		return nil
	}

	log.Printf("Found %d orphaned firewalls that will be deleted", len(orphanedFirewalls))

	for _, firewall := range orphanedFirewalls {
		log.Printf("Deleting orphaned firewall: %s (ID: %s)", firewall.Name, firewall.ID)

		err := m.Delete(ctx, firewall.ID)
		if err != nil {
			log.Printf("Failed to delete orphaned firewall %s: %v", firewall.Name, err)
			continue
		}

		log.Printf("Successfully deleted orphaned firewall: %s", firewall.Name)
	}

	return nil
}

// DeleteForCluster deletes firewall associated with a cluster
func (m *Manager) DeleteForCluster(ctx context.Context, clusterDef types.ClusterDefinition, firewallID string) error {
	// Delete firewall by ID if we have it stored
	if firewallID != "" {
		err := m.Delete(ctx, firewallID)
		if err != nil {
			log.Printf("Failed to delete firewall %s: %v", firewallID, err)
		} else {
			log.Printf("Deleted firewall %s", firewallID)
		}
		return err
	}

	// Try to find and delete firewall by name if we don't have the ID stored
	firewallName := fmt.Sprintf("%s-firewall", clusterDef.Metadata.Name)
	err := m.DeleteByName(ctx, firewallName)
	if err != nil {
		log.Printf("Failed to delete firewall %s: %v", firewallName, err)
	} else {
		log.Printf("Deleted firewall %s", firewallName)
	}

	return err
}
