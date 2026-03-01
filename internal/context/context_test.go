package context

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	mgr, err := NewManager()
	require.NoError(t, err)
	return mgr
}

func TestNewManager(t *testing.T) {
	mgr := newTestManager(t)
	require.NotNil(t, mgr)
	assert.NotEmpty(t, mgr.contextPath)
	assert.NotNil(t, mgr.context)
}

func TestNewManager_EmptyContext(t *testing.T) {
	mgr := newTestManager(t)
	assert.Empty(t, mgr.GetAWSAccount())
	assert.Empty(t, mgr.GetGCPProject())
	assert.Empty(t, mgr.GetAzureSubscription())
	assert.Empty(t, mgr.GetCivoOrganization())
}

func TestSetAndGetAWSAccount(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.SetAWSAccount("prod")
	require.NoError(t, err)
	assert.Equal(t, "prod", mgr.GetAWSAccount())
}

func TestSetAndGetGCPProject(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.SetGCPProject("my-project")
	require.NoError(t, err)
	assert.Equal(t, "my-project", mgr.GetGCPProject())
}

func TestSetAndGetAzureSubscription(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.SetAzureSubscription("prod-sub")
	require.NoError(t, err)
	assert.Equal(t, "prod-sub", mgr.GetAzureSubscription())
}

func TestSetAndGetCivoOrganization(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.SetCivoOrganization("my-org")
	require.NoError(t, err)
	assert.Equal(t, "my-org", mgr.GetCivoOrganization())
}

func TestGetCurrentAccount(t *testing.T) {
	mgr := newTestManager(t)

	require.NoError(t, mgr.SetAWSAccount("aws-prod"))
	require.NoError(t, mgr.SetGCPProject("gcp-dev"))
	require.NoError(t, mgr.SetAzureSubscription("azure-prod"))
	require.NoError(t, mgr.SetCivoOrganization("civo-default"))

	assert.Equal(t, "aws-prod", mgr.GetCurrentAccount("aws"))
	assert.Equal(t, "gcp-dev", mgr.GetCurrentAccount("gcp"))
	assert.Equal(t, "azure-prod", mgr.GetCurrentAccount("azure"))
	assert.Equal(t, "civo-default", mgr.GetCurrentAccount("civo"))
	assert.Empty(t, mgr.GetCurrentAccount("unknown"))
}

func TestSetCurrentAccount(t *testing.T) {
	mgr := newTestManager(t)

	require.NoError(t, mgr.SetCurrentAccount("aws", "prod"))
	require.NoError(t, mgr.SetCurrentAccount("gcp", "my-project"))
	require.NoError(t, mgr.SetCurrentAccount("azure", "my-sub"))
	require.NoError(t, mgr.SetCurrentAccount("civo", "my-org"))

	assert.Equal(t, "prod", mgr.GetAWSAccount())
	assert.Equal(t, "my-project", mgr.GetGCPProject())
	assert.Equal(t, "my-sub", mgr.GetAzureSubscription())
	assert.Equal(t, "my-org", mgr.GetCivoOrganization())
}

func TestSetCurrentAccount_UnknownProvider(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.SetCurrentAccount("unknown", "value")
	assert.Error(t, err)
}

func TestGetContext(t *testing.T) {
	mgr := newTestManager(t)

	require.NoError(t, mgr.SetAWSAccount("prod"))
	ctx := mgr.GetContext()
	require.NotNil(t, ctx)
	assert.Equal(t, "prod", ctx.AWS.Account)
}

func TestClear(t *testing.T) {
	mgr := newTestManager(t)

	require.NoError(t, mgr.SetAWSAccount("prod"))
	require.NoError(t, mgr.SetGCPProject("dev"))

	err := mgr.Clear()
	require.NoError(t, err)

	assert.Empty(t, mgr.GetAWSAccount())
	assert.Empty(t, mgr.GetGCPProject())
}

func TestClearProvider(t *testing.T) {
	mgr := newTestManager(t)

	require.NoError(t, mgr.SetAWSAccount("prod"))
	require.NoError(t, mgr.SetGCPProject("dev"))

	err := mgr.ClearProvider("aws")
	require.NoError(t, err)

	assert.Empty(t, mgr.GetAWSAccount())
	assert.Equal(t, "dev", mgr.GetGCPProject()) // GCP unchanged
}

func TestClearProvider_AllProviders(t *testing.T) {
	providers := []string{"aws", "gcp", "azure", "civo"}
	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			mgr := newTestManager(t)
			require.NoError(t, mgr.SetCurrentAccount(p, "test-value"))
			require.NoError(t, mgr.ClearProvider(p))
			assert.Empty(t, mgr.GetCurrentAccount(p))
		})
	}
}

func TestClearProvider_UnknownProvider(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.ClearProvider("unknown")
	assert.Error(t, err)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	require.NoError(t, mgr.SetAWSAccount("prod"))
	require.NoError(t, mgr.SetGCPProject("my-project"))

	// Reload from disk
	mgr2, err := NewManager()
	require.NoError(t, err)

	assert.Equal(t, "prod", mgr2.GetAWSAccount())
	assert.Equal(t, "my-project", mgr2.GetGCPProject())
}

func TestContextFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mgr, err := NewManager()
	require.NoError(t, err)

	expected := filepath.Join(tmpDir, ".hyve", "context.yaml")
	assert.Equal(t, expected, mgr.contextPath)
}

func TestGetSwitchCommands(t *testing.T) {
	tests := []struct {
		provider  string
		accountID string
		wantEmpty bool
	}{
		{"aws", "123456789012", false},
		{"gcp", "my-project-id", false},
		{"azure", "subscription-id", false},
		{"civo", "org-123", false},
		{"unknown", "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cmds := GetSwitchCommands(tt.provider, tt.accountID)
			if tt.wantEmpty {
				assert.Empty(t, cmds.CheckCommand)
				assert.Empty(t, cmds.SwitchCommand)
			} else {
				assert.NotEmpty(t, cmds.CheckCommand)
				assert.NotEmpty(t, cmds.SwitchCommand)
			}
		})
	}
}

func TestGetSwitchCommands_ContainsAccountID(t *testing.T) {
	cmds := GetSwitchCommands("gcp", "my-project")
	assert.Contains(t, cmds.SwitchCommand, "my-project")

	cmds = GetSwitchCommands("azure", "my-sub-id")
	assert.Contains(t, cmds.SwitchCommand, "my-sub-id")
}
