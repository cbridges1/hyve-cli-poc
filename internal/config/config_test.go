package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	require.NotNil(t, manager)
}
