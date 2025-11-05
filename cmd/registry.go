package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ServiceRegistration represents a service registration in the registry
type ServiceRegistration struct {
	Context    string                 `json:"@context"`
	Type       string                 `json:"@type"`
	Identifier string                 `json:"identifier"`
	Name       string                 `json:"name"`
	URL        string                 `json:"url"`
	Properties map[string]interface{} `json:"additionalProperty,omitempty"`
}

var (
	serviceIdentifier string
	httpClient        = &http.Client{Timeout: 10 * time.Second}
)

// registerWithRegistry registers this service as a semantic service
func registerWithRegistry(ctx context.Context, serviceURL, registryURL string) error {
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			hostname = "graphdb-service"
		}
	}

	serviceIdentifier = fmt.Sprintf("graphdb-service-%s", hostname)

	registration := ServiceRegistration{
		Context:    "https://schema.org",
		Type:       "SoftwareApplication",
		Identifier: serviceIdentifier,
		Name:       fmt.Sprintf("GraphDB Service - %s", hostname),
		URL:        serviceURL,
		Properties: map[string]interface{}{
			"version":    "2.0.0",
			"hostname":   hostname,
			"serviceType": "graphdb",
			"capabilities": []string{
				"graphdb-migration",
				"graphdb-create",
				"graphdb-delete",
				"graphdb-rename",
				"graphdb-import",
				"graphdb-export",
				"graph-migration",
				"graph-import",
				"graph-export",
				"graph-delete",
				"graph-rename",
			},
			"semanticEndpoint": fmt.Sprintf("%s/v1/api/semantic/action", serviceURL),
			"healthEndpoint":   fmt.Sprintf("%s/health", serviceURL),
		},
	}

	payload, err := json.Marshal(registration)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}

	registrationURL := fmt.Sprintf("%s/v1/api/services", registryURL)
	req, err := http.NewRequestWithContext(ctx, "POST", registrationURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	return nil
}

// startRegistryHeartbeat sends periodic heartbeats to the registry service
func startRegistryHeartbeat(ctx context.Context, serviceURL, registryURL string) {
	heartbeatInterval := 30 * time.Second
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sendRegistryHeartbeat(ctx, serviceURL, registryURL); err != nil {
				log.Printf("Failed to send registry heartbeat: %v", err)
			}

		case <-ctx.Done():
			log.Println("Registry heartbeat stopped")
			return
		}
	}
}

// sendRegistryHeartbeat sends a heartbeat to update service status
func sendRegistryHeartbeat(ctx context.Context, serviceURL, registryURL string) error {
	heartbeatURL := fmt.Sprintf("%s/v1/api/services/%s/heartbeat", registryURL, serviceIdentifier)

	heartbeat := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "healthy",
		"metrics": map[string]interface{}{
			"uptime": time.Since(time.Now()).Seconds(), // Will be properly tracked in production
		},
	}

	payload, err := json.Marshal(heartbeat)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", heartbeatURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If service not found (404), re-register
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Service not found in registry, re-registering...")
		return registerWithRegistry(ctx, serviceURL, registryURL)
	}

	return nil
}

// deregisterFromRegistry removes this service from the registry
func deregisterFromRegistry(ctx context.Context, serviceURL, registryURL string) error {
	deregisterURL := fmt.Sprintf("%s/v1/api/services/%s", registryURL, serviceIdentifier)

	req, err := http.NewRequestWithContext(ctx, "DELETE", deregisterURL, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("Deregistered service %s from registry", serviceIdentifier)
	return nil
}
