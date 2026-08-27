package tests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	Openapi "github.com/openshift/backplane-api/pkg/client"
)

// requireStagingEnv returns an error naming the first unset staging credential, so
// callers can t.Skip cleanly instead of failing CI when no staging creds are present.
func requireStagingEnv() error {
	for _, name := range []string{"BACKPLANE_TOKEN", "CLUSTER_ID", "BACKPLANE_API_URL"} {
		if os.Getenv(name) == "" {
			return fmt.Errorf("%s environment variable must be set", name)
		}
	}
	return nil
}

// instanceIDPattern matches the documented instanceId format: {name}--{uuid4}.
var instanceIDPattern = regexp.MustCompile(`^.+--[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// TestCreateTrustedAction creates a trusted action through the generated client and
// asserts the response carries proxyUri, a well-formed instanceId, and an expiry.
//
// It talks to a real staging backplane-api, so it skips (rather than fails) when the
// staging credentials are not present, keeping it inert in CI without creds.
func TestCreateTrustedAction(t *testing.T) {
	client, clusterID := newTrustedActionClient(t)
	ctx := context.Background()

	req := Openapi.CreateTrustedActionRequest{
		Name:               "integration-test",
		CustomerDataAccess: false,
		Rbac: Openapi.TrustedActionRbacDecl{
			ClusterRoleRules: []Openapi.PolicyRule{
				{
					Verbs:     &[]string{"get", "list"},
					ApiGroups: &[]string{""},
					Resources: &[]string{"pods"},
				},
			},
			Roles: []Openapi.RoleRbacDecl{},
		},
	}

	resp, err := client.CreateTrustedActionWithResponse(ctx, clusterID, nil, req)
	if err != nil {
		t.Fatalf("Failed to create trusted action: %v", err)
	}

	if resp.StatusCode() != 200 {
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil {
		t.Fatal("Expected trusted action in response body, got nil")
	}

	result := resp.JSON200

	if result.ProxyUri == "" {
		t.Error("Expected non-empty proxyUri in response")
	}

	if result.InstanceId == "" {
		t.Error("Expected non-empty instanceId in response")
	} else if !instanceIDPattern.MatchString(result.InstanceId) {
		t.Errorf("instanceId %q does not match expected format {name}--{uuid}", result.InstanceId)
	}

	if result.Expiry.IsZero() {
		t.Error("Expected a non-zero expiry in response")
	}

	t.Logf("Created trusted action: instanceId=%s proxyUri=%s expiry=%s",
		result.InstanceId, result.ProxyUri, result.Expiry)
}

// TestDeleteTrustedAction creates a trusted action then immediately deletes it,
// asserting 200 on delete. It also asserts that deleting an unknown instanceId returns 404.
func TestDeleteTrustedAction(t *testing.T) {
	client, clusterID := newTrustedActionClient(t)
	ctx := context.Background()

	req := Openapi.CreateTrustedActionRequest{
		Name:               "integration-test-delete",
		CustomerDataAccess: false,
		Rbac: Openapi.TrustedActionRbacDecl{
			ClusterRoleRules: []Openapi.PolicyRule{
				{
					Verbs:     &[]string{"get", "list"},
					ApiGroups: &[]string{""},
					Resources: &[]string{"pods"},
				},
			},
			Roles: []Openapi.RoleRbacDecl{},
		},
	}

	createResp, err := client.CreateTrustedActionWithResponse(ctx, clusterID, nil, req)
	if err != nil {
		t.Fatalf("Failed to create trusted action: %v", err)
	}
	if createResp.StatusCode() != 200 {
		t.Fatalf("Expected status 200 on create, got %d. Body: %s", createResp.StatusCode(), string(createResp.Body))
	}
	instanceID := createResp.JSON200.InstanceId

	deleteResp, err := client.DeleteTrustedActionWithResponse(ctx, clusterID, instanceID)
	if err != nil {
		t.Fatalf("Failed to delete trusted action: %v", err)
	}
	if deleteResp.StatusCode() != 200 {
		t.Fatalf("Expected status 200 on delete, got %d. Body: %s", deleteResp.StatusCode(), string(deleteResp.Body))
	}

	notFoundResp, err := client.DeleteTrustedActionWithResponse(ctx, clusterID, "nonexistent--00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("Failed to call delete on unknown instance: %v", err)
	}
	if notFoundResp.StatusCode() != 404 {
		t.Fatalf("Expected status 404 for unknown instanceId, got %d. Body: %s", notFoundResp.StatusCode(), string(notFoundResp.Body))
	}
}

// newTrustedActionClient builds an authenticated client for the trusted actions tests,
// skipping the test when any of the staging environment variables are unset so the
// suite does not fail in CI without credentials.
func newTrustedActionClient(t *testing.T) (*Openapi.ClientWithResponses, string) {
	t.Helper()

	if err := requireStagingEnv(); err != nil {
		t.Skipf("Skipping trusted actions integration test: %v", err)
	}

	return newAuthenticatedClient(t)
}
