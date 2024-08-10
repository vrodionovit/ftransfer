package main

import (
	"os"
	"reflect"
	"testing"
)

func TestReadConfig(t *testing.T) {
	// Create a temporary YAML file for testing
	yamlContent := `
connections:
  - name: "test_connection"
    host: "localhost"
    port: 22
    protocol: "sftp"
    username: "user"
    password: "pass"
    delay: 5
    path: "/remote/path"
    depth: 2
    regex: ".*"
`
	tmpFile, err := os.CreateTemp("", "connections.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Test readConfig function
	config, err := readConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("readConfig failed: %v", err)
	}

	expected := Config{
		Connections: []Connection{
			{
				Name:     "test_connection",
				Host:     "localhost",
				Port:     22,
				Protocol: "sftp",
				Username: "user",
				Password: "pass",
				Delay:    5,
				Path:     "/remote/path",
				Depth:    2,
				Regex:    ".*",
			},
		},
	}

	if !reflect.DeepEqual(config, expected) {
		t.Errorf("Expected %v, got %v", expected, config)
	}
}

func TestSplitConnections(t *testing.T) {
	config := Config{
		Connections: []Connection{
			{Name: "conn1"},
			{Name: "conn2"},
			{Name: "conn3"},
			{Name: "conn4"},
			{Name: "conn5"},
		},
	}

	splitted := splitConnections(config, 2)

	if len(splitted.Connection) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(splitted.Connection))
	}

	if len(splitted.Connection["group_1"]) != 2 {
		t.Errorf("Expected 2 connections in group_1, got %d", len(splitted.Connection["group_1"]))
	}

	if len(splitted.Connection["group_2"]) != 3 {
		t.Errorf("Expected 3 connections in group_2, got %d", len(splitted.Connection["group_2"]))
	}
}
