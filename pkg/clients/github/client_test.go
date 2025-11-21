/*
 * Copyright 2025 TomTom N.V.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestClient creates a test Client that uses a mock HTTP server instead of the real GitHub API
func createTestClient(t *testing.T, server *httptest.Server) *Client {
	// Create a dummy private key for testing
	privateKeyFile, cleanup := createTestPrivateKey(t)
	t.Cleanup(cleanup)

	// Create a client that points to our test server instead of the real GitHub API
	// This requires some modifications to the original client to make it testable
	httpClient := &http.Client{
		Transport: &mockTransport{
			server:         server,
			originalClient: http.DefaultClient,
		},
	}

	// We can't directly modify the internal http client of github.Client
	// So we'll need to use go-github's NewClient function and replace the transport
	ghClient := github.NewClient(httpClient)
	// Override the BaseURL to point to our test server
	ghClient.BaseURL = mustParseURL(server.URL + "/")

	client := &Client{
		ghClient:   ghClient,
		restClient: httpClient,
		config: ClientConfig{
			AppID:          123,
			InstallationID: 456,
			PrivateKeyFile: privateKeyFile,
			Organization:   "test-org",
		},
	}

	return client
}

// mockTransport is a custom http.RoundTripper that redirects GitHub API requests to our test server
type mockTransport struct {
	server         *httptest.Server
	originalClient *http.Client
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the GitHub API URL with our test server URL
	if strings.HasPrefix(req.URL.String(), "https://api.github.com") {
		newURL := strings.Replace(req.URL.String(), "https://api.github.com", t.server.URL, 1)
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		// Copy headers
		newReq.Header = req.Header
		return t.originalClient.Do(newReq)
	}
	return t.originalClient.Do(req)
}

func mustParseURL(rawURL string) *url.URL {
	u, _ := url.Parse(rawURL)
	return u
}

// setupTestServer creates a test HTTP server that mocks GitHub API responses
func setupTestServer() (*httptest.Server, *http.ServeMux) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	return server, mux
}

// createTestPrivateKey creates a temporary test private key file
func createTestPrivateKey(t *testing.T) (string, func()) {
	// This is a dummy EC private key for testing purposes
	dummyPrivateKey := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIH+p+Mo+PKUYlFPHhQQfI9uxKJqYjQz3RyUiBhe7jJMDoAoGCCqGSM49
AwEHoUQDQgAEDXMYtpuJ1tN6/NHjYdEO5r8WR+ygNh4EeY8KwuLTMl0zvGwfvjVB
M5kVRQxH9jWkLQ6UYHO6+3HUEuOQbsIFQA==
-----END EC PRIVATE KEY-----`

	tmpFile, err := os.CreateTemp("", "test-private-key-*.pem")
	require.NoError(t, err)

	_, err = tmpFile.WriteString(dummyPrivateKey)
	require.NoError(t, err)

	err = tmpFile.Close()
	require.NoError(t, err)

	return tmpFile.Name(), func() {
		os.Remove(tmpFile.Name())
	}
}

// TestNew tests the New function with various configurations
func TestNew(t *testing.T) {
	// Test with missing required fields
	testCases := []struct {
		name   string
		errMsg string
		config ClientConfig
	}{
		{
			name:   "Missing AppID",
			config: ClientConfig{InstallationID: 123, PrivateKeyFile: "key.pem", Organization: "org"},
			errMsg: "config parameter AppID is required",
		},
		{
			name:   "Missing InstallationID",
			config: ClientConfig{AppID: 123, PrivateKeyFile: "key.pem", Organization: "org"},
			errMsg: "config parameter InstallationID is required",
		},
		{
			name:   "Missing Organization",
			config: ClientConfig{AppID: 123, InstallationID: 456, PrivateKeyFile: "key.pem"},
			errMsg: "config parameter Organization is required",
		},
		{
			name:   "Missing PrivateKey and PrivateKeyFile",
			config: ClientConfig{AppID: 123, InstallationID: 456, Organization: "org"},
			errMsg: "either PrivateKey or PrivateKeyFile must be provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(tc.config)
			assert.Nil(t, client)
			assert.EqualError(t, err, tc.errMsg)
		})
	}

	// We can't fully test successful initialization without valid credentials,
	// but we can test that it attempts to use the provided configuration
	privateKeyFile, cleanup := createTestPrivateKey(t)
	defer cleanup()

	_, err := New(ClientConfig{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyFile: privateKeyFile,
		Organization:   "test-org",
	})

	// This should still fail because we're using fake credentials,
	// but it should try to use the private key file
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize GitHub transport")
}

// TestGetRunnerRegistrationToken tests the GetRunnerRegistrationToken method
func TestGetRunnerRegistrationToken(t *testing.T) {
	server, mux := setupTestServer()
	defer server.Close()

	// Set up the test route for creating a registration token
	mux.HandleFunc("/orgs/test-org/actions/runners/registration-token", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)

		// Return a successful response with a test token
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		expiration := time.Now().Add(1 * time.Hour)
		fmt.Fprintf(w, `{
			"token": "test-token-12345",
			"expires_at": "%s"
		}`, expiration.Format(time.RFC3339))
	})

	// Create a client that uses our test server
	client := createTestClient(t, server)

	// Call the method we're testing
	token, err := client.GetRunnerRegistrationToken()

	// Verify the result
	require.NoError(t, err)
	assert.Equal(t, "test-token-12345", *token.Token)
	assert.NotNil(t, token.ExpiresAt)
}

// TestGetRunnerByName tests the GetRunnerByName method
func TestGetRunnerByName(t *testing.T) {
	server, mux := setupTestServer()
	defer server.Close()

	// Set up the test route for getting runners by name
	mux.HandleFunc("/orgs/test-org/actions/runners", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "test-runner", r.URL.Query().Get("name"))

		// Return a successful response with a test runner
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"total_count": 1,
			"runners": [
				{
					"id": 123,
					"name": "test-runner",
					"os": "linux",
					"status": "online"
				}
			]
		}`)
	})

	// Create a client that uses our test server
	client := createTestClient(t, server)

	// Test with empty runner name (should error)
	_, err := client.GetRunnerByName("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runner name cannot be empty")

	// Test with valid runner name
	runners, err := client.GetRunnerByName("test-runner")

	// Verify the result
	require.NoError(t, err)
	assert.Equal(t, 1, runners.TotalCount)
	require.Len(t, runners.Runners, 1)
	assert.Equal(t, int64(123), *runners.Runners[0].ID)
	assert.Equal(t, "test-runner", *runners.Runners[0].Name)
	assert.Equal(t, "online", *runners.Runners[0].Status)
}

// TestGetRunnerByName_Error tests error scenarios for GetRunnerByName
func TestGetRunnerByName_Error(t *testing.T) {
	server, mux := setupTestServer()
	defer server.Close()

	// Set up the test route to return an error
	mux.HandleFunc("/orgs/test-org/actions/runners", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"message": "Not Found"}`)
	})

	// Create a client that uses our test server
	client := createTestClient(t, server)

	// Call the method with a non-existent runner
	_, err := client.GetRunnerByName("non-existent-runner")

	// Verify we get an error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get GitHub runner by name")
	assert.Contains(t, err.Error(), "status code 404")
}

// TestRemoveRunner tests the RemoveRunner method
func TestRemoveRunner(t *testing.T) {
	server, mux := setupTestServer()
	defer server.Close()

	// Set up the test route for removing a runner
	mux.HandleFunc("/orgs/test-org/actions/runners/123", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)

		// Return a successful response (no content)
		w.WriteHeader(http.StatusNoContent)
	})

	// Create a client that uses our test server
	client := createTestClient(t, server)

	// Test with invalid runner ID (should error)
	err := client.RemoveRunner(0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove runner")

	// Test with valid runner ID
	err = client.RemoveRunner(123)

	// Verify the result
	assert.NoError(t, err)
}

// TestRemoveRunner_Error tests error scenarios for RemoveRunner
func TestRemoveRunner_Error(t *testing.T) {
	server, mux := setupTestServer()
	defer server.Close()

	// Set up the test route to return an error
	mux.HandleFunc("/orgs/test-org/actions/runners/456", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)

		// Return an error response
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"message": "Runner not found"}`)
	})

	// Create a client that uses our test server
	client := createTestClient(t, server)

	// Call the method with a non-existent runner ID
	err := client.RemoveRunner(456)

	// Verify we get an error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove runner 456")
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client := createTestClient(t, server)

	// Currently, Close() is a no-op, so we just verify it doesn't return an error
	err := client.Close()
	assert.NoError(t, err)
}
