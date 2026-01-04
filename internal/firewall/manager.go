package firewall

import (
	"context"
	"fmt"
	"log"
	"strings"

	"civo-cluster-deploy/internal/provider"
	"civo-cluster-deploy/internal/types"
)

// Manager handles firewall operations using a generic provider
type Manager struct {
	provider provider.FirewallProvider
}

// NewManager creates a new firewall manager
func NewManager(p provider.Provider) *Manager {
	return &Manager{
		provider: p,
	}
}

// CreateFromDefinition creates or reuses a firewall based on cluster definition
func (m *Manager) CreateFromDefinition(ctx context.Context, clusterDef types.ClusterDefinition) (*provider.Firewall, error) {
	firewallName := clusterDef.Metadata.Name + "-firewall"

	// Check if firewall already exists
	existingFirewall, err := m.provider.FindFirewallByName(ctx, firewallName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing firewall: %w", err)
	}

	if existingFirewall != nil {
		log.Printf("Firewall %s already exists with ID %s, reusing it", firewallName, existingFirewall.ID)
		return existingFirewall, nil
	}

	// Convert types.FirewallRule to provider.FirewallRule
	var rules []provider.FirewallRule
	for _, rule := range clusterDef.Spec.Firewall.Rules {
		rules = append(rules, provider.FirewallRule{
			Protocol:  rule.Protocol,
			StartPort: rule.StartPort,
			EndPort:   rule.EndPort,
			Cidr:      rule.Cidr,
			Direction: rule.Direction,
		})
	}

	firewallConfig := &provider.FirewallConfig{
		Name:  firewallName,
		Rules: rules,
	}

	log.Printf("Creating firewall %s", firewallName)
	return m.provider.CreateFirewall(ctx, firewallConfig)
}

// FindByName finds a firewall by name
func (m *Manager) FindByName(ctx context.Context, name string) (*provider.Firewall, error) {
	return m.provider.FindFirewallByName(ctx, name)
}

// DeleteForCluster deletes a firewall associated with a cluster
func (m *Manager) DeleteForCluster(ctx context.Context, clusterDef types.ClusterDefinition, firewallID string) error {
	if firewallID == "" {
		log.Printf("No firewall ID provided for cluster %s, skipping firewall deletion", clusterDef.Metadata.Name)
		return nil
	}

	log.Printf("Deleting firewall %s for cluster %s", firewallID, clusterDef.Metadata.Name)

	err := m.provider.DeleteFirewall(ctx, firewallID)
	if err != nil {
		return fmt.Errorf("failed to delete firewall %s: %w", firewallID, err)
	}

	log.Printf("Successfully deleted firewall %s", firewallID)
	return nil
}

// FindOrphaned finds firewalls that exist but are not associated with any desired clusters
func (m *Manager) FindOrphaned(ctx context.Context, desiredClusters []types.ClusterDefinition) ([]*provider.Firewall, error) {
	allFirewalls, err := m.provider.ListFirewalls(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list firewalls: %w", err)
	}

	// Create a map of expected firewall names
	expectedNames := make(map[string]bool)
	for _, cluster := range desiredClusters {
		if cluster.Spec.Firewall.Enabled {
			firewallName := cluster.Metadata.Name + "-firewall"
			expectedNames[firewallName] = true
		}
	}

	var orphaned []*provider.Firewall
	for _, firewall := range allFirewalls {
		if m.ShouldManage(*firewall) && !expectedNames[firewall.Name] {
			orphaned = append(orphaned, firewall)
		}
	}

	return orphaned, nil
}

// ShouldManage determines if we should manage this firewall
func (m *Manager) ShouldManage(firewall provider.Firewall) bool {
	managedSuffixes := []string{"-firewall"}
	managedPrefixes := []string{"hyve-", "civo-deploy-"}

	for _, suffix := range managedSuffixes {
		if strings.HasSuffix(firewall.Name, suffix) {
			// Check if it has our managed prefix
			for _, prefix := range managedPrefixes {
				if strings.HasPrefix(firewall.Name, prefix) {
					return true
				}
			}
			// If it ends with -firewall but doesn't have our prefix, still manage it
			// for backward compatibility
			return true
		}
	}

	return false
}

// CleanupOrphaned removes orphaned firewalls
func (m *Manager) CleanupOrphaned(ctx context.Context, orphanedFirewalls []*provider.Firewall) error {
	for _, firewall := range orphanedFirewalls {
		log.Printf("Cleaning up orphaned firewall: %s (ID: %s)", firewall.Name, firewall.ID)

		err := m.provider.DeleteFirewall(ctx, firewall.ID)
		if err != nil {
			log.Printf("Failed to delete orphaned firewall %s: %v", firewall.Name, err)
			continue
		}

		log.Printf("Successfully deleted orphaned firewall: %s", firewall.Name)
	}

	return nil
}
