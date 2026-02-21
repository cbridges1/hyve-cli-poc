package config

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}
}
