//go:build integration
// +build integration

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"eve.evalgo.org/db"
	"github.com/labstack/echo/v4"
)

const (
	sourceGraphDBURL = "http://localhost:7201"
	targetGraphDBURL = "http://localhost:7202"
	defaultUsername  = "admin"
	defaultPassword  = "root"
	testTimeout      = 60 * time.Second
)

// waitForGraphDB waits for GraphDB instance to be ready
func waitForGraphDB(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/rest/repositories")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			t.Logf("GraphDB at %s is ready", url)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("GraphDB at %s did not become ready within %v", url, timeout)
}

// setupTestRepository creates a test repository with sample data
func setupTestRepository(t *testing.T, url, username, password, repoName string) {
	t.Helper()

	// Create TTL configuration for test repository
	config := fmt.Sprintf(`@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rep: <http://www.openrdf.org/config/repository#> .
@prefix sr: <http://www.openrdf.org/config/repository/sail#> .
@prefix sail: <http://www.openrdf.org/config/sail#> .
@prefix graphdb: <http://www.ontotext.com/config/graphdb#> .

[] a rep:Repository ;
   rep:repositoryID "%s" ;
   rdfs:label "%s Test Repository" ;
   rep:repositoryImpl [
      rep:repositoryType "graphdb:SailRepository" ;
      sr:sailImpl [
         sail:sailType "graphdb:Sail" ;
         graphdb:read-only "false" ;
         graphdb:ruleset "empty" ;
      ]
   ] .`, repoName, repoName)

	// Write config to temp file
	tmpFile, err := os.CreateTemp("", "repo-config-*.ttl")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(config); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Create repository
	db.HttpClient = http.DefaultClient
	err = db.GraphDBRestoreConf(url, username, password, tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create repository %s: %v", repoName, err)
	}

	t.Logf("Created repository: %s at %s", repoName, url)
}

// insertTestData inserts sample RDF data into a repository
func insertTestData(t *testing.T, url, username, password, repo string, triples int) {
	t.Helper()

	// Generate sample RDF data
	var rdfData strings.Builder
	rdfData.WriteString("@prefix ex: <http://example.org/> .\n")
	rdfData.WriteString("@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .\n\n")

	for i := 0; i < triples; i++ {
		rdfData.WriteString(fmt.Sprintf("ex:entity%d a ex:TestEntity ;\n", i))
		rdfData.WriteString(fmt.Sprintf("    rdfs:label \"Test Entity %d\" ;\n", i))
		rdfData.WriteString(fmt.Sprintf("    ex:value %d .\n\n", i))
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "test-data-*.ttl")
	if err != nil {
		t.Fatalf("failed to create temp data file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(rdfData.String()); err != nil {
		t.Fatalf("failed to write data file: %v", err)
	}
	tmpFile.Close()

	// Import data
	db.HttpClient = http.DefaultClient
	err = db.GraphDBImportGraphRdf(url, username, password, repo, "", tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	t.Logf("Inserted %d triples into repository: %s", triples, repo)
}

// cleanupTestRepository deletes a test repository
func cleanupTestRepository(t *testing.T, url, username, password, repoName string) {
	t.Helper()
	db.HttpClient = http.DefaultClient
	err := db.GraphDBDeleteRepository(url, username, password, repoName)
	if err != nil {
		t.Logf("Warning: failed to cleanup repository %s: %v", repoName, err)
	} else {
		t.Logf("Cleaned up repository: %s", repoName)
	}
}

// TestIntegrationSetup verifies the test environment is properly configured
func TestIntegrationSetup(t *testing.T) {
	t.Log("Verifying GraphDB 10.8.5 test instances...")

	// Wait for all GraphDB instances
	waitForGraphDB(t, sourceGraphDBURL, testTimeout)
	waitForGraphDB(t, targetGraphDBURL, testTimeout)

	// Verify we can list repositories
	for _, url := range []string{sourceGraphDBURL, targetGraphDBURL} {
		db.HttpClient = http.DefaultClient
		repos, err := db.GraphDBRepositories(url, defaultUsername, defaultPassword)
		if err != nil {
			t.Fatalf("failed to list repositories at %s: %v", url, err)
		}
		t.Logf("GraphDB at %s has %d repositories", url, len(repos.Results.Bindings))
	}
}

// TestIntegrationRepoMigration tests complete repository migration
func TestIntegrationRepoMigration(t *testing.T) {
	t.Log("Testing repository migration...")

	// Setup
	waitForGraphDB(t, sourceGraphDBURL, testTimeout)
	waitForGraphDB(t, targetGraphDBURL, testTimeout)

	sourceRepo := "test-migration-source"
	targetRepo := "test-migration-target"

	// Create source repository with data
	setupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo)
	defer cleanupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo)

	insertTestData(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo, 100)

	// Ensure target repository doesn't exist
	cleanupTestRepository(t, targetGraphDBURL, defaultUsername, defaultPassword, targetRepo)
	defer cleanupTestRepository(t, targetGraphDBURL, defaultUsername, defaultPassword, targetRepo)

	// Create request
	req := MigrationRequest{
		Version: "v0.0.1",
		Tasks: []Task{
			{
				Action: "repo-migration",
				Src: &Repository{
					URL:      sourceGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     sourceRepo,
				},
				Tgt: &Repository{
					URL:      targetGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     targetRepo,
				},
			},
		},
	}

	// Execute migration
	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/api/action", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", "test-key")

	_ = os.Setenv("API_KEY", "test-key")
	defer os.Unsetenv("API_KEY")

	w := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(httpReq, w)

	err := migrationHandlerJSON(c)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 but got %d: %s", w.Code, w.Body.String())
	}

	// Verify target repository exists
	db.HttpClient = http.DefaultClient
	repos, err := db.GraphDBRepositories(targetGraphDBURL, defaultUsername, defaultPassword)
	if err != nil {
		t.Fatalf("failed to list target repositories: %v", err)
	}

	found := false
	for _, binding := range repos.Results.Bindings {
		if binding.Id["value"] == targetRepo {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("target repository %s was not created", targetRepo)
	}

	t.Log("Repository migration completed successfully")
}

// TestIntegrationGraphMigration tests graph migration between repositories
func TestIntegrationGraphMigration(t *testing.T) {
	t.Log("Testing graph migration...")

	// Setup
	waitForGraphDB(t, sourceGraphDBURL, testTimeout)
	waitForGraphDB(t, targetGraphDBURL, testTimeout)

	sourceRepo := "test-graph-migration-source"
	targetRepo := "test-graph-migration-target"
	testGraph := "http://example.org/test-graph"

	// Create repositories
	setupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo)
	defer cleanupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo)

	setupTestRepository(t, targetGraphDBURL, defaultUsername, defaultPassword, targetRepo)
	defer cleanupTestRepository(t, targetGraphDBURL, defaultUsername, defaultPassword, targetRepo)

	// Insert data into source graph
	insertTestData(t, sourceGraphDBURL, defaultUsername, defaultPassword, sourceRepo+"/"+testGraph, 50)

	// Create request
	req := MigrationRequest{
		Version: "v0.0.1",
		Tasks: []Task{
			{
				Action: "graph-migration",
				Src: &Repository{
					URL:      sourceGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     sourceRepo,
					Graph:    testGraph,
				},
				Tgt: &Repository{
					URL:      targetGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     targetRepo,
					Graph:    testGraph,
				},
			},
		},
	}

	// Execute migration
	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/api/action", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", "test-key")

	_ = os.Setenv("API_KEY", "test-key")
	defer os.Unsetenv("API_KEY")

	w := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(httpReq, w)

	err := migrationHandlerJSON(c)
	if err != nil {
		t.Fatalf("graph migration failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 but got %d: %s", w.Code, w.Body.String())
	}

	// Verify target graph exists
	db.HttpClient = http.DefaultClient
	graphs, err := db.GraphDBListGraphs(targetGraphDBURL, defaultUsername, defaultPassword, targetRepo)
	if err != nil {
		t.Fatalf("failed to list target graphs: %v", err)
	}

	found := false
	for _, binding := range graphs.Results.Bindings {
		if binding.ContextID.Value == testGraph {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("target graph %s was not created", testGraph)
	}

	t.Log("Graph migration completed successfully")
}

// TestIntegrationRepoCreateAndDelete tests repository creation and deletion
func TestIntegrationRepoCreateAndDelete(t *testing.T) {
	t.Log("Testing repository create and delete...")

	waitForGraphDB(t, targetGraphDBURL, testTimeout)

	testRepo := "test-create-delete"

	// Cleanup first
	cleanupTestRepository(t, targetGraphDBURL, defaultUsername, defaultPassword, testRepo)

	// Create TTL config
	config := fmt.Sprintf(`@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix rep: <http://www.openrdf.org/config/repository#> .
@prefix sr: <http://www.openrdf.org/config/repository/sail#> .
@prefix sail: <http://www.openrdf.org/config/sail#> .
@prefix graphdb: <http://www.ontotext.com/config/graphdb#> .

[] a rep:Repository ;
   rep:repositoryID "%s" ;
   rdfs:label "Test Repository" ;
   rep:repositoryImpl [
      rep:repositoryType "graphdb:SailRepository" ;
      sr:sailImpl [
         sail:sailType "graphdb:Sail" ;
         graphdb:read-only "false" ;
         graphdb:ruleset "empty" ;
      ]
   ] .`, testRepo)

	// Create multipart request for repo-create
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add request JSON
	reqData := MigrationRequest{
		Version: "v0.0.1",
		Tasks: []Task{
			{
				Action: "repo-create",
				Tgt: &Repository{
					URL:      targetGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     testRepo,
				},
			},
		},
	}
	reqJSON, _ := json.Marshal(reqData)
	_ = writer.WriteField("request", string(reqJSON))

	// Add config file
	configPart, _ := writer.CreateFormFile("task_0_config", "repo-config.ttl")
	_, _ = io.WriteString(configPart, config)
	writer.Close()

	// Send create request
	httpReq := httptest.NewRequest("POST", "/v1/api/action", body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("x-api-key", "test-key")

	_ = os.Setenv("API_KEY", "test-key")
	defer os.Unsetenv("API_KEY")

	w := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(httpReq, w)

	err := migrationHandler(c)
	if err != nil {
		t.Fatalf("repo create failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 but got %d: %s", w.Code, w.Body.String())
	}

	// Verify repository exists
	db.HttpClient = http.DefaultClient
	repos, err := db.GraphDBRepositories(targetGraphDBURL, defaultUsername, defaultPassword)
	if err != nil {
		t.Fatalf("failed to list repositories: %v", err)
	}

	found := false
	for _, binding := range repos.Results.Bindings {
		if binding.Id["value"] == testRepo {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("repository %s was not created", testRepo)
	}

	t.Log("Repository created successfully")

	// Now test deletion
	deleteReq := MigrationRequest{
		Version: "v0.0.1",
		Tasks: []Task{
			{
				Action: "repo-delete",
				Tgt: &Repository{
					URL:      targetGraphDBURL,
					Username: defaultUsername,
					Password: defaultPassword,
					Repo:     testRepo,
				},
			},
		},
	}

	reqBody, _ := json.Marshal(deleteReq)
	httpReq2 := httptest.NewRequest("POST", "/v1/api/action", bytes.NewReader(reqBody))
	httpReq2.Header.Set("Content-Type", "application/json")
	httpReq2.Header.Set("x-api-key", "test-key")

	w2 := httptest.NewRecorder()
	c2 := e.NewContext(httpReq2, w2)

	err = migrationHandlerJSON(c2)
	if err != nil {
		t.Fatalf("repo delete failed: %v", err)
	}

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 but got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify repository is deleted
	repos2, err := db.GraphDBRepositories(targetGraphDBURL, defaultUsername, defaultPassword)
	if err != nil {
		t.Fatalf("failed to list repositories after delete: %v", err)
	}

	for _, binding := range repos2.Results.Bindings {
		if binding.Id["value"] == testRepo {
			t.Fatalf("repository %s was not deleted", testRepo)
		}
	}

	t.Log("Repository deleted successfully")
}

// TestIntegrationMultipleRepos tests migrations with multiple repositories
func TestIntegrationMultipleRepos(t *testing.T) {
	t.Log("Testing multiple repository operations...")

	waitForGraphDB(t, sourceGraphDBURL, testTimeout)
	waitForGraphDB(t, targetGraphDBURL, testTimeout)

	repo1 := "test-multi-repo-1"
	repo2 := "test-multi-repo-2"

	// Create repositories
	setupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo1)
	defer cleanupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo1)

	setupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo2)
	defer cleanupTestRepository(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo2)

	// Insert data into both repositories
	insertTestData(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo1, 30)
	insertTestData(t, sourceGraphDBURL, defaultUsername, defaultPassword, repo2, 40)

	// Test listing multiple repositories
	db.HttpClient = http.DefaultClient
	repos, err := db.GraphDBRepositories(sourceGraphDBURL, defaultUsername, defaultPassword)
	if err != nil {
		t.Fatalf("failed to list repositories: %v", err)
	}

	foundCount := 0
	for _, binding := range repos.Results.Bindings {
		repoName := binding.Id["value"]
		if repoName == repo1 || repoName == repo2 {
			foundCount++
		}
	}

	if foundCount < 2 {
		t.Fatalf("expected to find at least 2 test repositories, found %d", foundCount)
	}

	t.Logf("Multiple repository operations completed successfully (found %d test repos)", foundCount)
}
