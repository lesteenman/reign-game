package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// fakeSSM captures the parameter name requested and returns a
// configured value or error. It records whether it was called at all,
// so the env-var-wins test can assert the SSM path is never hit.
type fakeSSM struct {
	called   bool
	wantName string
	value    string
	err      error
}

func (f *fakeSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	f.called = true
	f.wantName = ""
	if in != nil && in.Name != nil {
		f.wantName = *in.Name
	}
	if f.err != nil {
		return nil, f.err
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{Value: &f.value},
	}, nil
}

func TestLoadClerkSecretWithDeps_EnvVarSet_SkipsSSM(t *testing.T) {
	// Arrange
	fake := &fakeSSM{value: "sk_test_from_ssm"}
	env := func(k string) string {
		switch k {
		case "CLERK_SECRET_KEY":
			return "sk_test_from_env"
		case "CLERK_SECRET_PARAM_NAME":
			return "/reign/prod/clerk-secret-key"
		}
		return ""
	}

	// Act
	got, err := loadClerkSecretWithDeps(context.Background(), env, func(context.Context) (ssmGetter, error) {
		return fake, nil
	})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sk_test_from_env" {
		t.Errorf("secret = %q, want %q", got, "sk_test_from_env")
	}
	if fake.called {
		t.Error("ssm GetParameter was called even though CLERK_SECRET_KEY was set")
	}
}

func TestLoadClerkSecretWithDeps_EnvVarEmpty_FetchesFromSSM(t *testing.T) {
	// Arrange
	fake := &fakeSSM{value: "sk_test_from_ssm"}
	env := func(k string) string {
		if k == "CLERK_SECRET_PARAM_NAME" {
			return "/reign/prod/clerk-secret-key"
		}
		return ""
	}

	// Act
	got, err := loadClerkSecretWithDeps(context.Background(), env, func(context.Context) (ssmGetter, error) {
		return fake, nil
	})

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "sk_test_from_ssm" {
		t.Errorf("secret = %q, want %q", got, "sk_test_from_ssm")
	}
	if !fake.called {
		t.Error("ssm GetParameter was not called")
	}
	if fake.wantName != "/reign/prod/clerk-secret-key" {
		t.Errorf("ssm parameter name = %q, want %q", fake.wantName, "/reign/prod/clerk-secret-key")
	}
}

func TestLoadClerkSecretWithDeps_BothUnset_ReturnsError(t *testing.T) {
	// Arrange
	env := func(string) string { return "" }

	// Act
	_, err := loadClerkSecretWithDeps(context.Background(), env, func(context.Context) (ssmGetter, error) {
		t.Fatal("ssm client constructor should not be called when both env vars are empty")
		return nil, nil
	})

	// Assert
	if err == nil {
		t.Fatal("expected error when both CLERK_SECRET_KEY and CLERK_SECRET_PARAM_NAME are unset")
	}
	if !strings.Contains(err.Error(), "CLERK_SECRET_KEY") {
		t.Errorf("error should mention the env var names, got: %v", err)
	}
}

func TestLoadClerkSecretWithDeps_SSMError_ReturnsWrappedError(t *testing.T) {
	// Arrange
	fake := &fakeSSM{err: errors.New("ssm unavailable")}
	env := func(k string) string {
		if k == "CLERK_SECRET_PARAM_NAME" {
			return "/reign/prod/clerk-secret-key"
		}
		return ""
	}

	// Act
	_, err := loadClerkSecretWithDeps(context.Background(), env, func(context.Context) (ssmGetter, error) {
		return fake, nil
	})

	// Assert
	if err == nil {
		t.Fatal("expected error when SSM GetParameter fails")
	}
	if !strings.Contains(err.Error(), "ssm unavailable") {
		t.Errorf("error should wrap the underlying SSM error, got: %v", err)
	}
}

func TestLoadClerkSecretWithDeps_SSMReturnsEmptyValue_ReturnsError(t *testing.T) {
	// Arrange
	fake := &fakeSSM{value: ""}
	env := func(k string) string {
		if k == "CLERK_SECRET_PARAM_NAME" {
			return "/reign/prod/clerk-secret-key"
		}
		return ""
	}

	// Act
	_, err := loadClerkSecretWithDeps(context.Background(), env, func(context.Context) (ssmGetter, error) {
		return fake, nil
	})

	// Assert
	if err == nil {
		t.Fatal("expected error when SSM returns an empty value")
	}
}
