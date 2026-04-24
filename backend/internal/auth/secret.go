package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// ssmGetter is the narrow subset of the SSM client surface that the
// secret loader depends on. Defined locally so tests can substitute a
// fake without pulling in the real SDK client or mocking its internals.
type ssmGetter interface {
	GetParameter(ctx context.Context, in *ssm.GetParameterInput, opts ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// ssmClientFactory returns an SSM client. Tests substitute this to
// avoid creating a real AWS client or hitting the network.
type ssmClientFactory func(ctx context.Context) (ssmGetter, error)

// defaultSSMClientFactory loads the default AWS SDK config and
// returns a live SSM client. Used in the exported LoadClerkSecret
// entry point.
func defaultSSMClientFactory(ctx context.Context) (ssmGetter, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for SSM: %w", err)
	}
	return ssm.NewFromConfig(cfg), nil
}

var (
	secretLoadOnce sync.Once
	cachedSecret   string
	cachedErr      error
)

// LoadClerkSecret returns the Clerk secret key for this process,
// fetched once and cached for the lifetime of the container.
//
// Resolution order:
//  1. CLERK_SECRET_KEY — if set, used directly. This is the local dev
//     path; backend/.env.local populates it. The SSM client is never
//     constructed and the network is never touched.
//  2. CLERK_SECRET_PARAM_NAME — if set, treated as the SSM Parameter
//     Store path (e.g. /reign/prod/clerk-secret-key) and fetched with
//     decryption. This is the Lambda path — the env var holds only the
//     parameter name, never the secret itself, because Lambda env vars
//     are readable via lambda:GetFunctionConfiguration.
//  3. Neither set — returns an error so the caller fails hard at
//     startup (log.Fatal-class).
//
// The sync.Once guarantees the underlying fetch runs at most once per
// container. Subsequent calls return the cached value.
func LoadClerkSecret(ctx context.Context) (string, error) {
	secretLoadOnce.Do(func() {
		cachedSecret, cachedErr = loadClerkSecretWithDeps(ctx, os.Getenv, defaultSSMClientFactory)
	})
	return cachedSecret, cachedErr
}

// loadClerkSecretWithDeps is the testable inner implementation. It
// takes the env-var lookup and SSM client factory as parameters so
// tests can inject fakes for the two resolution branches.
func loadClerkSecretWithDeps(ctx context.Context, getenv func(string) string, newSSM ssmClientFactory) (string, error) {
	if v := getenv("CLERK_SECRET_KEY"); v != "" {
		log.Printf("auth secret: using CLERK_SECRET_KEY from env (dev path)")
		return v, nil
	}

	paramName := getenv("CLERK_SECRET_PARAM_NAME")
	if paramName == "" {
		return "", errors.New("clerk secret not configured: neither CLERK_SECRET_KEY nor CLERK_SECRET_PARAM_NAME is set")
	}

	client, err := newSSM(ctx)
	if err != nil {
		return "", fmt.Errorf("auth secret: constructing SSM client: %w", err)
	}

	withDecryption := true
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: &withDecryption,
	})
	if err != nil {
		return "", fmt.Errorf("auth secret: fetching SSM parameter %q: %w", paramName, err)
	}
	if out == nil || out.Parameter == nil || out.Parameter.Value == nil || *out.Parameter.Value == "" {
		return "", fmt.Errorf("auth secret: SSM parameter %q returned empty value", paramName)
	}

	log.Printf("auth secret: loaded from SSM param=%s", paramName)
	return *out.Parameter.Value, nil
}

// Enforce that the real *ssm.Client implements our narrow interface.
// Fails at compile time if AWS ever changes the surface — cheap
// insurance against a silent breakage we'd otherwise only notice
// when a Lambda cold-start paniced in production.
var _ ssmGetter = (*ssm.Client)(nil)
