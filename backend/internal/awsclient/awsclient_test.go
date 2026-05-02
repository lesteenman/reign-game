package awsclient

import (
	"context"
	"os"
	"testing"
)

func TestLoadAWSConfig_DefaultRegion(t *testing.T) {
	// Arrange
	t.Setenv("AWS_REGION", "")

	// Act
	cfg, err := LoadAWSConfig(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("expected default region us-east-1, got %s", cfg.Region)
	}
}

func TestLoadAWSConfig_HonorsAWSRegionEnv(t *testing.T) {
	// Arrange
	t.Setenv("AWS_REGION", "eu-west-1")

	// Act
	cfg, err := LoadAWSConfig(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Fatalf("expected eu-west-1, got %s", cfg.Region)
	}
}

func TestNewDynamoDBClient_DoesNotPanic(t *testing.T) {
	// Arrange
	cfg, err := LoadAWSConfig(context.Background())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Act
	client := NewDynamoDBClient(&cfg)

	// Assert
	if client == nil {
		t.Fatalf("expected non-nil client")
	}
	_ = os.Unsetenv("DYNAMODB_ENDPOINT")
}
