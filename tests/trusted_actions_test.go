package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	t.Cleanup(func() {
		delResp, _ := client.DeleteTrustedActionWithResponse(ctx, clusterID, result.InstanceId)
		if delResp != nil && delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
			t.Logf("Warning: cleanup failed to delete instanceId=%s, status=%d", result.InstanceId, delResp.StatusCode())
		}
	})

	t.Logf("Created trusted action: instanceId=%s proxyUri=%s expiry=%s",
		result.InstanceId, result.ProxyUri, result.Expiry)
}

// TestDeleteTrustedAction creates a trusted action then immediately deletes it,
// asserting 200 on delete. It also asserts that deleting an unknown instanceId returns 404.
func TestDeleteTrustedAction(t *testing.T) {
	client, clusterID := newTrustedActionClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	t.Cleanup(func() {
		delResp, _ := client.DeleteTrustedActionWithResponse(ctx, clusterID, instanceID)
		if delResp != nil && delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
			t.Logf("Warning: cleanup failed to delete instanceId=%s, status=%d", instanceID, delResp.StatusCode())
		}
	})

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

// TestProxyTrustedAction creates a trusted action, then issues a proxied GET to
// /api/v1/namespaces through the returned proxyUri and asserts a successful (2xx) response.
func TestProxyTrustedAction(t *testing.T) {
	client, clusterID := newTrustedActionClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := Openapi.CreateTrustedActionRequest{
		Name:               "integration-test-proxy",
		CustomerDataAccess: false,
		Rbac: Openapi.TrustedActionRbacDecl{
			ClusterRoleRules: []Openapi.PolicyRule{
				{
					Verbs:     &[]string{"get", "list"},
					ApiGroups: &[]string{""},
					Resources: &[]string{"namespaces"},
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

	t.Cleanup(func() {
		delResp, _ := client.DeleteTrustedActionWithResponse(ctx, clusterID, instanceID)
		if delResp != nil && delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
			t.Logf("Warning: cleanup failed to delete instanceId=%s, status=%d", instanceID, delResp.StatusCode())
		}
	})

	proxyResp, err := client.GetBackplaneTrustedactionClusterIdTrustedActionInstanceId(ctx, clusterID, instanceID, func(ctx context.Context, req *http.Request) error {
		req.URL.Path = strings.TrimRight(req.URL.Path, "/") + "/api/v1/namespaces"
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to make proxied K8s call: %v", err)
	}
	defer proxyResp.Body.Close()

	if proxyResp.StatusCode < 200 || proxyResp.StatusCode >= 300 {
		t.Errorf("Expected 2xx proxied response, got %d", proxyResp.StatusCode)
	}

	t.Logf("Proxied K8s call to /api/v1/namespaces succeeded: instanceId=%s status=%d", instanceID, proxyResp.StatusCode)
}

// TestProxyTrustedActionMutations creates a trusted action with permissions to create
// and update ConfigMaps, then tests POST, PATCH, and PUT operations through the proxy.
func TestProxyTrustedActionMutations(t *testing.T) {
	client, clusterID := newTrustedActionClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create trusted action with permissions to manage ConfigMaps
	req := Openapi.CreateTrustedActionRequest{
		Name:               "integration-test-mutations",
		CustomerDataAccess: false,
		Rbac: Openapi.TrustedActionRbacDecl{
			ClusterRoleRules: []Openapi.PolicyRule{
				{
					Verbs:     &[]string{"get", "list", "create", "update", "patch", "delete"},
					ApiGroups: &[]string{""},
					Resources: &[]string{"configmaps"},
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

	t.Cleanup(func() {
		delResp, _ := client.DeleteTrustedActionWithResponse(ctx, clusterID, instanceID)
		if delResp != nil && delResp.StatusCode() != 200 && delResp.StatusCode() != 404 {
			t.Logf("Warning: cleanup failed to delete instanceId=%s, status=%d", instanceID, delResp.StatusCode())
		}
	})

	// Test POST: Create a ConfigMap
	configMapJSON := `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "test-proxy-mutation",
			"namespace": "default"
		},
		"data": {
			"key1": "value1"
		}
	}`

	postResp, err := client.PostBackplaneTrustedactionClusterIdTrustedActionInstanceIdWithBody(
		ctx, clusterID, instanceID, "application/json",
		bytes.NewBufferString(configMapJSON),
		func(ctx context.Context, req *http.Request) error {
			req.URL.Path = strings.TrimRight(req.URL.Path, "/") + "/api/v1/namespaces/default/configmaps"
			return nil
		})
	if err != nil {
		t.Fatalf("Failed to POST ConfigMap through proxy: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode < 200 || postResp.StatusCode >= 300 {
		body, _ := io.ReadAll(postResp.Body)
		t.Fatalf("Expected 2xx response for POST, got %d. Body: %s", postResp.StatusCode, string(body))
	}
	io.Copy(io.Discard, postResp.Body)
	t.Logf("POST through proxy succeeded: status=%d", postResp.StatusCode)

	// Test PATCH: Update the ConfigMap
	patchJSON := `{
		"data": {
			"key1": "updated-value",
			"key2": "new-value"
		}
	}`

	patchResp, err := client.PatchBackplaneTrustedactionClusterIdTrustedActionInstanceIdWithBody(
		ctx, clusterID, instanceID, "application/merge-patch+json",
		bytes.NewBufferString(patchJSON),
		func(ctx context.Context, req *http.Request) error {
			req.URL.Path = strings.TrimRight(req.URL.Path, "/") + "/api/v1/namespaces/default/configmaps/test-proxy-mutation"
			return nil
		})
	if err != nil {
		t.Fatalf("Failed to PATCH ConfigMap through proxy: %v", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode < 200 || patchResp.StatusCode >= 300 {
		body, _ := io.ReadAll(patchResp.Body)
		t.Errorf("Expected 2xx response for PATCH, got %d. Body: %s", patchResp.StatusCode, string(body))
	} else {
		t.Logf("PATCH through proxy succeeded: status=%d", patchResp.StatusCode)
	}

	// Test PUT: Replace the ConfigMap
	putConfigMapJSON := `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "test-proxy-mutation",
			"namespace": "default"
		},
		"data": {
			"key3": "replaced-value"
		}
	}`

	putResp, err := client.PutBackplaneTrustedactionClusterIdTrustedActionInstanceIdWithBody(
		ctx, clusterID, instanceID, "application/json",
		bytes.NewBufferString(putConfigMapJSON),
		func(ctx context.Context, req *http.Request) error {
			req.URL.Path = strings.TrimRight(req.URL.Path, "/") + "/api/v1/namespaces/default/configmaps/test-proxy-mutation"
			return nil
		})
	if err != nil {
		t.Fatalf("Failed to PUT ConfigMap through proxy: %v", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		body, _ := io.ReadAll(putResp.Body)
		t.Errorf("Expected 2xx response for PUT, got %d. Body: %s", putResp.StatusCode, string(body))
	} else {
		t.Logf("PUT through proxy succeeded: status=%d", putResp.StatusCode)
	}

	// Clean up: Delete the ConfigMap
	deleteResp, err := client.DeleteBackplaneTrustedactionClusterIdTrustedActionInstanceId(
		ctx, clusterID, instanceID,
		func(ctx context.Context, req *http.Request) error {
			req.URL.Path = strings.TrimRight(req.URL.Path, "/") + "/api/v1/namespaces/default/configmaps/test-proxy-mutation"
			return nil
		})
	if err != nil {
		t.Fatalf("Failed to DELETE ConfigMap through proxy: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode < 200 || deleteResp.StatusCode >= 300 {
		body, _ := io.ReadAll(deleteResp.Body)
		t.Logf("Warning: ConfigMap cleanup failed with status %d. Body: %s", deleteResp.StatusCode, string(body))
	}

	t.Logf("Mutation tests completed: instanceId=%s", instanceID)
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
