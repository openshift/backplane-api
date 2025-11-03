# Cluster Reports API Integration Tests

This document describes how to run integration tests for the new cluster reports API endpoints against your staging environment.

## Overview

The integration test suite validates the following API endpoints:
- `GET /backplane/cluster/{cluster_id}/reports` - List reports for a cluster
- `POST /backplane/cluster/{cluster_id}/reports` - Create a new report
- `GET /backplane/cluster/{cluster_id}/reports/{report_id}` - Get a specific report

## Prerequisites

1. Go 1.22.5 or later
2. Access to the staging environment
3. A valid authentication token
4. A cluster ID to test against

## Setup

### 1. Set Environment Variables

```bash
# Required: Your authentication token for the staging environment
export BACKPLANE_TOKEN="your-token-here"

# Required: The cluster UUID to test against
export CLUSTER_ID="your-cluster-id-here"

# Required: API URL
export BACKPLANE_API_URL="https://backplane-api.staging.example.com"

# Optional: HTTP/HTTPS Proxy configuration (required for backplane-api access)
export PROXY="http://proxy.example.com:8080"
```

### 2. Proxy Configuration

The backplane-api typically requires proxy access. The test suite supports standard HTTP/HTTPS proxy environment variables:

- **HTTP_PROXY**: The proxy server URL (e.g., `http://proxy.corp.com:8080`)

The test will automatically detect and use these proxy settings. If `HTTP_PROXY` is set, you'll see a log message indicating the proxy being used:

```
Using proxy: http://proxy.example.com:8080
```

### 3. Get Your Token

You can obtain your token from your staging environment authentication system. The token should have appropriate permissions to create and read cluster reports.

### 4. Get a Cluster ID

You need a valid cluster UUID from your staging environment. You can get this from your cluster management system or OCM.

## Running the Tests

### Run All Tests

```bash
go test -v ./tests/reports_test.go
```

### Run a Specific Test

```bash
# Test creating a report
go test -v -run TestCreateReport ./tests/reports_test.go

# Test listing reports
go test -v -run TestListReports ./tests/reports_test.go

# Test end-to-end workflow
go test -v -run TestReportsEndToEnd ./tests/reports_test.go
```

### Run with Timeout

```bash
go test -v -timeout 30s ./tests/reports_test.go
```

### Run with Verbose Output

```bash
go test -v -count=1 ./tests/reports_test.go
```

## Test Descriptions

### TestCreateReport
Creates a new report with test data, verifies it's created successfully (201 status), retrieves it by ID, and validates that the data round-trips correctly (create → retrieve → decode → verify).

**What it tests:**
- Creating a report with valid data
- Base64 encoding/decoding of report content
- Retrieving a report by ID
- Data integrity across create/retrieve cycle

### TestListReports
Tests the list endpoint with different parameters.

**Sub-tests:**
- `ListAllReports` - Lists all reports for the cluster
- `ListLimitedReports` - Tests the `last` query parameter to limit results

**What it tests:**
- Listing all reports for a cluster
- Query parameter handling
- Response pagination/limiting

### TestGetNonExistentReport
Attempts to retrieve a report that doesn't exist.

**What it tests:**
- Proper 404 error handling
- Error response format

### TestCreateInvalidReport
Tests validation by attempting to create reports with invalid data.

**Sub-tests:**
- `EmptySummary` - Report with empty summary field
- `EmptyData` - Report with empty data field

**What it tests:**
- Input validation
- Error handling (400/422 responses)
- Required field validation

### TestReportsEndToEnd
Complete workflow test that performs all operations in sequence.

**Workflow:**
1. Create a report
2. Retrieve the specific report by ID
3. List all reports and verify the created report appears in the list

**What it tests:**
- Complete API workflow
- Data consistency across endpoints
- Integration between create, get, and list operations

## Expected Output

Successful test run:

```
=== RUN   TestCreateReport
    integration_test.go:95: Successfully created report with ID: abc123...
    integration_test.go:96: Report summary: Integration Test Report - 2025-11-03T...
    integration_test.go:97: Created at: 2025-11-03T...
    integration_test.go:132: Report data verified successfully
--- PASS: TestCreateReport (1.23s)
=== RUN   TestListReports
=== RUN   TestListReports/ListAllReports
    integration_test.go:149: Found 5 reports for cluster
--- PASS: TestListReports (0.45s)
    --- PASS: TestListReports/ListAllReports (0.22s)
    --- PASS: TestListReports/ListLimitedReports (0.23s)
=== RUN   TestGetNonExistentReport
    integration_test.go:216: Correctly received 404 for non-existent report
--- PASS: TestGetNonExistentReport (0.12s)
=== RUN   TestCreateInvalidReport
=== RUN   TestCreateInvalidReport/EmptySummary
    integration_test.go:246: Correctly rejected empty summary with status 400
=== RUN   TestCreateInvalidReport/EmptyData
    integration_test.go:263: Correctly rejected empty data with status 400
--- PASS: TestCreateInvalidReport (0.24s)
    --- PASS: TestCreateInvalidReport/EmptySummary (0.12s)
    --- PASS: TestCreateInvalidReport/EmptyData (0.12s)
=== RUN   TestReportsEndToEnd
    integration_test.go:301: Created report: def456...
    integration_test.go:309: Retrieved report: def456...
    integration_test.go:328: Verified report def456... appears in list
    integration_test.go:331: End-to-end test completed successfully
--- PASS: TestReportsEndToEnd (1.45s)
PASS
ok      command-line-arguments  3.494s
```

## Troubleshooting

### Authentication Errors (401/403)

```
Error: Expected status 201, got 401
```

**Solution:** Verify your `BACKPLANE_TOKEN` is valid and has not expired.

### Cluster Not Found (404)

```
Error: Expected status 201, got 404
```

**Solution:** Verify your `CLUSTER_ID` exists in the staging environment.

### Connection Errors

```
Error: Failed to create API client: ... connection refused
```

**Solution:**
- Check your `BACKPLANE_API_URL` is correct
- Verify you have network access to the staging environment
- Check if the staging environment is running

### Missing Environment Variables

```
Error: BACKPLANE_TOKEN environment variable must be set
```

**Solution:** Make sure all required environment variables are set before running the tests.

## Notes

- These tests create actual data in your staging environment
- Tests are designed to be idempotent but may accumulate test reports over time
- Each test run creates new reports; they are not automatically cleaned up
- The tests use real API calls and will count against any rate limits
- Test execution time varies based on network latency and API response times

## Support

If you encounter issues:
1. Check the troubleshooting section above
2. Verify your environment variables are correct
3. Ensure the staging environment is accessible
4. Check API logs for detailed error messages
