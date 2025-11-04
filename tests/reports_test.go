package tests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	Openapi "github.com/openshift/backplane-api/pkg/client"
)

func getStagingConfig(t *testing.T) (string, string, string) {
	token := os.Getenv("BACKPLANE_TOKEN")
	if token == "" {
		t.Fatal("BACKPLANE_TOKEN environment variable must be set")
	}

	clusterID := os.Getenv("CLUSTER_ID")
	if clusterID == "" {
		t.Fatal("CLUSTER_ID environment variable must be set")
	}

	apiURL := os.Getenv("BACKPLANE_API_URL")
	if apiURL == "" {
		t.Fatal("BACKPLANE_API_URL environment variable must be set")
	}

	return apiURL, token, clusterID
}

func createHTTPClient(t *testing.T) *http.Client {
	transport := &http.Transport{}

	// Configure proxy if environment variables are set
	proxyURL := os.Getenv("PROXY")
	if proxyURL != "" {
		parsedProxyURL, err := url.Parse(proxyURL)
		if err != nil {
			t.Fatalf("Failed to parse proxy URL '%s': %v", proxyURL, err)
		}
		transport.Proxy = http.ProxyURL(parsedProxyURL)
		t.Logf("Using proxy: %s", proxyURL)
	} else {
		// Use standard proxy detection from environment
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(30) * time.Second, // 30 second timeout
	}
}

func newAuthenticatedClient(t *testing.T) (*Openapi.ClientWithResponses, string) {
	apiURL, token, clusterID := getStagingConfig(t)

	// Create HTTP client with proxy support
	httpClient := createHTTPClient(t)

	// Create request editor function for authentication
	authFunc := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	}

	client, err := Openapi.NewClientWithResponses(
		apiURL,
		Openapi.WithHTTPClient(httpClient),
		Openapi.WithRequestEditorFn(authFunc),
	)
	if err != nil {
		t.Fatalf("Failed to create API client: %v", err)
	}

	return client, clusterID
}

// TestCreateReport tests creating a new cluster report
func TestCreateReport(t *testing.T) {
	client, clusterID := newAuthenticatedClient(t)
	ctx := context.Background()

	// Create test report data
	reportData := map[string]interface{}{
		"test":      true,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Integration test report",
	}

	// Convert to JSON and base64 encode
	jsonData, err := json.Marshal(reportData)
	if err != nil {
		t.Fatalf("Failed to marshal report data: %v", err)
	}
	encodedData := base64.StdEncoding.EncodeToString(jsonData)

	// Create the report
	createReq := Openapi.CreateReport{
		Summary: "Integration Test Report - " + time.Now().Format(time.RFC3339),
		Data:    encodedData,
	}

	resp, err := client.CreateReportWithResponse(ctx, clusterID, createReq)
	if err != nil {
		t.Fatalf("Failed to create report: %v", err)
	}

	if resp.StatusCode() != 201 {
		t.Errorf("Expected status 201, got %d. Body: %s", resp.StatusCode(), string(resp.Body))
		return
	}

	if resp.JSON201 == nil {
		t.Fatal("Expected report in response body, got nil")
	}

	t.Logf("Successfully created report with ID: %s", resp.JSON201.ReportId)
	t.Logf("Report summary: %s", resp.JSON201.Summary)
	t.Logf("Created at: %s", resp.JSON201.CreatedAt.Format(time.RFC3339))

	// Verify we can retrieve the report
	getResp, err := client.GetReportByIdWithResponse(ctx, clusterID, resp.JSON201.ReportId)
	if err != nil {
		t.Fatalf("Failed to get report by ID: %v", err)
	}

	if getResp.StatusCode() != 200 {
		t.Errorf("Expected status 200 when getting report, got %d", getResp.StatusCode())
		return
	}

	if getResp.JSON200 == nil {
		t.Fatal("Expected report in get response body, got nil")
	}

	if getResp.JSON200.ReportId != resp.JSON201.ReportId {
		t.Errorf("Report ID mismatch: created=%s, retrieved=%s", resp.JSON201.ReportId, getResp.JSON200.ReportId)
	}

	// Verify the data round-trips correctly
	decodedData, err := base64.StdEncoding.DecodeString(getResp.JSON200.Data)
	if err != nil {
		t.Fatalf("Failed to decode report data: %v", err)
	}

	var retrievedData map[string]interface{}
	if err := json.Unmarshal(decodedData, &retrievedData); err != nil {
		t.Fatalf("Failed to unmarshal retrieved data: %v", err)
	}

	if retrievedData["message"] != reportData["message"] {
		t.Errorf("Data mismatch: expected message=%s, got=%s", reportData["message"], retrievedData["message"])
	}

	t.Log("Report data verified successfully")
}

// TestListReports tests listing reports for a cluster
func TestListReports(t *testing.T) {
	client, clusterID := newAuthenticatedClient(t)
	ctx := context.Background()

	// Test listing all reports
	t.Run("ListAllReports", func(t *testing.T) {
		resp, err := client.GetReportsByClusterWithResponse(ctx, clusterID, nil)
		if err != nil {
			t.Fatalf("Failed to list reports: %v", err)
		}

		if resp.StatusCode() != 200 {
			t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode(), string(resp.Body))
			return
		}

		if resp.JSON200 == nil {
			t.Fatal("Expected reports list in response body, got nil")
		}

		t.Logf("Found %d reports for cluster", len(*resp.JSON200.Reports))

		if resp.JSON200.Reports != nil {
			for i, report := range *resp.JSON200.Reports {
				t.Logf("  Report %d: ID=%s, Summary=%s, Created=%s",
					i+1,
					*report.ReportId,
					*report.Summary,
					report.CreatedAt.Format(time.RFC3339))
			}
		}
	})

	// Test listing with limit
	t.Run("ListLimitedReports", func(t *testing.T) {
		limit := 5
		params := &Openapi.GetReportsByClusterParams{
			Last: &limit,
		}

		resp, err := client.GetReportsByClusterWithResponse(ctx, clusterID, params)
		if err != nil {
			t.Fatalf("Failed to list limited reports: %v", err)
		}

		if resp.StatusCode() != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode())
			return
		}

		if resp.JSON200 == nil {
			t.Fatal("Expected reports list in response body, got nil")
		}

		reportCount := 0
		if resp.JSON200.Reports != nil {
			reportCount = len(*resp.JSON200.Reports)
		}

		if reportCount > limit {
			t.Errorf("Expected at most %d reports, got %d", limit, reportCount)
		}

		t.Logf("Successfully retrieved %d reports (limit=%d)", reportCount, limit)
	})
}

// TestGetNonExistentReport tests retrieving a report that doesn't exist
func TestGetNonExistentReport(t *testing.T) {
	client, clusterID := newAuthenticatedClient(t)
	ctx := context.Background()

	fakeReportID := "nonexistent-report-id-12345"

	resp, err := client.GetReportByIdWithResponse(ctx, clusterID, fakeReportID)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// Should get 404 for non-existent report
	if resp.StatusCode() != 404 {
		t.Errorf("Expected status 404 for non-existent report, got %d", resp.StatusCode())
	}

	t.Logf("Correctly received 404 for non-existent report")
}

// TestCreateInvalidReport tests creating a report with invalid data
func TestCreateInvalidReport(t *testing.T) {
	client, clusterID := newAuthenticatedClient(t)
	ctx := context.Background()

	t.Run("EmptySummary", func(t *testing.T) {
		createReq := Openapi.CreateReport{
			Summary: "",
			Data:    "dGVzdA==", // base64("test")
		}

		resp, err := client.CreateReportWithResponse(ctx, clusterID, createReq)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		// Should get 400 for invalid input
		if resp.StatusCode() != 400 && resp.StatusCode() != 422 {
			t.Errorf("Expected status 400 or 422 for empty summary, got %d", resp.StatusCode())
		}

		t.Logf("Correctly rejected empty summary with status %d", resp.StatusCode())
	})

	t.Run("EmptyData", func(t *testing.T) {
		createReq := Openapi.CreateReport{
			Summary: "Test Report",
			Data:    "",
		}

		resp, err := client.CreateReportWithResponse(ctx, clusterID, createReq)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		// Should get 400 for invalid input
		if resp.StatusCode() != 400 && resp.StatusCode() != 422 {
			t.Errorf("Expected status 400 or 422 for empty data, got %d", resp.StatusCode())
		}

		t.Logf("Correctly rejected empty data with status %d", resp.StatusCode())
	})
}

// TestReportsEndToEnd performs a complete end-to-end test
func TestReportsEndToEnd(t *testing.T) {
	client, clusterID := newAuthenticatedClient(t)
	ctx := context.Background()

	// 1. Create a report
	reportData := map[string]interface{}{
		"test_type": "end_to_end",
		"timestamp": time.Now().Format(time.RFC3339),
		"results": map[string]interface{}{
			"tests_run":    10,
			"tests_passed": 8,
			"tests_failed": 2,
		},
	}

	jsonData, _ := json.Marshal(reportData)
	encodedData := base64.StdEncoding.EncodeToString(jsonData)

	createReq := Openapi.CreateReport{
		Summary: fmt.Sprintf("E2E Test - %s", time.Now().Format("2006-01-02 15:04:05")),
		Data:    encodedData,
	}

	createResp, err := client.CreateReportWithResponse(ctx, clusterID, createReq)
	if err != nil {
		t.Fatalf("Failed to create report: %v", err)
	}
	if createResp.StatusCode() != 201 {
		t.Fatalf("Create failed with status %d", createResp.StatusCode())
	}

	reportID := createResp.JSON201.ReportId
	t.Logf("Created report: %s", reportID)

	// 2. Retrieve the specific report
	getResp, err := client.GetReportByIdWithResponse(ctx, clusterID, reportID)
	if err != nil {
		t.Fatalf("Failed to get report: %v", err)
	}
	if getResp.StatusCode() != 200 {
		t.Fatalf("Get failed with status %d", getResp.StatusCode())
	}
	t.Logf("Retrieved report: %s", reportID)

	// 3. List reports and verify our report is in the list
	listResp, err := client.GetReportsByClusterWithResponse(ctx, clusterID, nil)
	if err != nil {
		t.Fatalf("Failed to list reports: %v", err)
	}
	if listResp.StatusCode() != 200 {
		t.Fatalf("List failed with status %d", listResp.StatusCode())
	}

	found := false
	if listResp.JSON200.Reports != nil {
		for _, report := range *listResp.JSON200.Reports {
			if *report.ReportId == reportID {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("Created report %s not found in list", reportID)
	} else {
		t.Logf("Verified report %s appears in list", reportID)
	}

	t.Log("End-to-end test completed successfully")
}
