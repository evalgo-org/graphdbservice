// Package helpers provides validation utilities.
package helpers

import (
	"fmt"
	"net/http"

	"eve.evalgo.org/db"
	"graphdbservice/internal/domain"
)

// ValidateRepositoryExists checks if a repository exists in GraphDB
func ValidateRepositoryExists(client *http.Client, url, username, password, repoName string) (bool, error) {
	db.HttpClient = client

	repos, err := db.GraphDBRepositories(url, username, password)
	if err != nil {
		return false, err
	}

	if repos.Results.Bindings == nil {
		return false, nil
	}

	for _, binding := range repos.Results.Bindings {
		if id, exists := binding["id"].(map[string]interface{}); exists {
			if value, ok := id["value"].(string); ok && value == repoName {
				return true, nil
			}
		}
	}

	return false, nil
}

// ValidateGraphExists checks if a graph exists in a repository
func ValidateGraphExists(client *http.Client, url, username, password, repoName, graphURI string) (bool, error) {
	db.HttpClient = client

	graphs, err := db.GraphDBListGraphs(url, username, password, repoName)
	if err != nil {
		return false, err
	}

	if graphs.Results.Bindings == nil {
		return false, nil
	}

	for _, binding := range graphs.Results.Bindings {
		if contextID, exists := binding["contextID"].(map[string]interface{}); exists {
			if value, ok := contextID["value"].(string); ok && value == graphURI {
				return true, nil
			}
		}
	}

	return false, nil
}

// ValidateRepositoryNotExists checks that a repository does NOT exist
func ValidateRepositoryNotExists(client *http.Client, url, username, password, repoName string) error {
	exists, err := ValidateRepositoryExists(client, url, username, password, repoName)
	if err != nil {
		return fmt.Errorf("failed to check if repository exists: %w", err)
	}

	if exists {
		return domain.NewConflictError("repository", repoName)
	}

	return nil
}

// ValidateGraphNotExists checks that a graph does NOT exist
func ValidateGraphNotExists(client *http.Client, url, username, password, repoName, graphURI string) error {
	exists, err := ValidateGraphExists(client, url, username, password, repoName, graphURI)
	if err != nil {
		return fmt.Errorf("failed to check if graph exists: %w", err)
	}

	if exists {
		return domain.NewConflictError("graph", graphURI)
	}

	return nil
}

// GetRepositoryNames extracts repository names from GraphDB API response bindings
func GetRepositoryNames(bindings []map[string]interface{}) []string {
	names := make([]string, 0)
	for _, binding := range bindings {
		if id, exists := binding["id"].(map[string]interface{}); exists {
			if value, ok := id["value"].(string); ok {
				names = append(names, value)
			}
		}
	}
	return names
}
