package cluster

import (
	"testing"
)

func TestIsWDSCluster(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"prefix_wds", "wds1", true},
		{"contains_dash", "prod-wds-2", true},
		{"contains_underscore", "test_wds_env", true},
		{"no_wds", "prodcluster", false},
		{"mixed_case", "WdsCluster", false}, // function likely case-sensitive
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWDSCluster(tt.input)
			if got != tt.expected {
				t.Errorf("isWDSCluster(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetTargetNamespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_returns_default", "", "default"},
		{"non_empty_returns_same", "kube-system", "kube-system"},
		{"spaces_not_trimmed", " myns ", " myns "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTargetNamespace(tt.input)
			if got != tt.expected {
				t.Errorf("GetTargetNamespace(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
