package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eve.evalgo.org/common"
	"eve.evalgo.org/config"
	evehttp "eve.evalgo.org/http"
	"eve.evalgo.org/registry"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Start GraphDB semantic service",
	Long: `Start the GraphDB service as a semantic service.

The service exposes GraphDB operations as Schema.org semantic actions:
  - TransferAction: Repository/graph migration
  - CreateAction: Repository creation
  - DeleteAction: Repository/graph deletion
  - UpdateAction: Repository/graph rename
  - UploadAction: Data import
  - ItemList: Batch workflows

The service registers with the registry service for discovery and can be
orchestrated by the 'when' scheduler and other semantic executors.

Environment Variables:
  - GRAPHDB_SERVICE_PORT: Port to listen on (default: 8080)
  - GRAPHDB_SERVICE_URL: Public URL of this service (default: http://hostname:port)
  - REGISTRYSERVICE_API_URL: Registry service URL (default: http://localhost:8096)
  - HOSTNAME: Hostname for service identification (default: system hostname)
  - API_KEY: Optional API key for endpoint protection`,
	Run: runSemanticService,
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	serviceCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serviceCmd.Flags().String("service-url", "", "Public URL of this service")
	serviceCmd.Flags().String("registry-url", "", "Registry service URL")
	serviceCmd.Flags().String("api-key", "", "API key for endpoint protection")
	serviceCmd.Flags().Bool("debug", false, "Enable debug logging")
}

func runSemanticService(cmd *cobra.Command, args []string) {
	// Load configuration using EVE config utilities
	env := config.NewEnvConfig("GRAPHDB")
	serverConfig := evehttp.DefaultServerConfig()
	serverConfig.Port = env.GetInt("SERVICE_PORT", 8080)
	serverConfig.Debug = env.GetBool("DEBUG", false)
	serverConfig.BodyLimit = "100M"

	// Service configuration
	serviceURL := env.GetString("SERVICE_URL", "")
	registryURL := env.GetString("REGISTRY_URL", "http://localhost:8096")
	apiKey := env.GetString("API_KEY", "")

	// Override from flags if provided
	if flagPort, _ := cmd.Flags().GetInt("port"); flagPort != 0 {
		serverConfig.Port = flagPort
	}
	if flagURL, _ := cmd.Flags().GetString("service-url"); flagURL != "" {
		serviceURL = flagURL
	}
	if flagRegistry, _ := cmd.Flags().GetString("registry-url"); flagRegistry != "" {
		registryURL = flagRegistry
	}
	if flagKey, _ := cmd.Flags().GetString("api-key"); flagKey != "" {
		apiKey = flagKey
	}
	if flagDebug, _ := cmd.Flags().GetBool("debug"); flagDebug {
		serverConfig.Debug = true
	}

	// Set debug mode globally
	debugMode = serverConfig.Debug

	// Determine service URL if not provided
	if serviceURL == "" {
		hostname := os.Getenv("HOSTNAME")
		if hostname == "" {
			var err error
			hostname, err = os.Hostname()
			if err != nil {
				hostname = "localhost"
			}
		}
		serviceURL = fmt.Sprintf("http://%s:%d", hostname, serverConfig.Port)
	}

	// Setup structured logging
	logger := common.ServiceLogger("graphdb-service", "2.0.0")
	logger.Info("=====================================")
	logger.Info("GraphDB Semantic Service Starting")
	logger.Info("=====================================")
	logger.WithFields(map[string]interface{}{
		"service_url":  serviceURL,
		"registry_url": registryURL,
		"port":         serverConfig.Port,
		"debug":        serverConfig.Debug,
		"api_key_set":  apiKey != "",
	}).Info("Configuration loaded")

	// Create Echo server with EVE http utilities
	e := evehttp.NewEchoServer(serverConfig)

	// Add security headers middleware
	e.Use(evehttp.SecurityHeadersMiddleware())

	// API key middleware (if configured)
	if apiKey != "" {
		// Semantic action endpoint with API key protection
		e.POST("/v1/api/semantic/action", handleSemanticAction, evehttp.APIKeyMiddleware(apiKey))
	} else {
		// Semantic action endpoint without protection
		e.POST("/v1/api/semantic/action", handleSemanticAction)
	}

	// Health check endpoint using EVE utilities (always public)
	e.GET("/health", evehttp.HealthCheckHandler("graphdb-semantic", "2.0.0"))

	// Start server in goroutine
	go func() {
		logger.WithFields(map[string]interface{}{
			"port":              serverConfig.Port,
			"semantic_endpoint": fmt.Sprintf("%s/v1/api/semantic/action", serviceURL),
			"health_endpoint":   fmt.Sprintf("%s/health", serviceURL),
		}).Info("Starting HTTP server")

		if err := evehttp.StartServer(e, serverConfig); err != nil {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Create registry client using EVE registry utilities
	registryClient := registry.NewClient(registry.ClientConfig{
		RegistryURL: registryURL,
		Timeout:     10 * time.Second,
	})

	// Register with registry service
	ctx := context.Background()
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
	}

	err := registryClient.Register(ctx, registry.ServiceConfig{
		ServiceID:   fmt.Sprintf("graphdb-service-%s", hostname),
		ServiceName: fmt.Sprintf("GraphDB Service - %s", hostname),
		ServiceURL:  serviceURL,
		Version:     "2.0.0",
		Hostname:    hostname,
		ServiceType: "graphdb",
		Capabilities: []string{
			"graphdb-migration", "graphdb-create", "graphdb-delete",
			"graphdb-rename", "graphdb-import", "graphdb-export",
			"graph-migration", "graph-import", "graph-export",
			"graph-delete", "graph-rename",
		},
		Properties: map[string]interface{}{
			"semanticEndpoint": fmt.Sprintf("%s/v1/api/semantic/action", serviceURL),
			"healthEndpoint":   fmt.Sprintf("%s/health", serviceURL),
		},
	})

	if err != nil {
		logger.WithError(err).Warn("Failed to register with registry, service will continue without registration")
	} else {
		logger.Info("Successfully registered with registry service")
		// Start heartbeat using registry client
		cancelHeartbeat := registryClient.StartHeartbeat(ctx, 30*time.Second)
		defer cancelHeartbeat()
	}

	logger.Info("Service is ready. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down service...")

	// Deregister from registry using EVE registry utilities
	if err := registryClient.Deregister(context.Background()); err != nil {
		logger.WithError(err).Warn("Failed to deregister from registry")
	} else {
		logger.Info("Successfully deregistered from registry")
	}

	// Shutdown server using EVE http utilities
	if err := evehttp.GracefulShutdown(e, serverConfig.ShutdownTimeout); err != nil {
		logger.WithError(err).Error("Error during graceful shutdown")
	}

	logger.Info("Service stopped")
}
