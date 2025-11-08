package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eve.evalgo.org/common"
	evehttp "eve.evalgo.org/http"
	"eve.evalgo.org/statemanager"
	"eve.evalgo.org/registry"
	"eve.evalgo.org/tracing"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
)

// Global state manager
var stateManager *statemanager.Manager

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
	// Load configuration from environment
	serverConfig := evehttp.DefaultServerConfig()
	serverConfig.Port = common.GetEnvInt("GRAPHDB_SERVICE_PORT", 8080)
	serverConfig.Debug = common.GetEnvBool("GRAPHDB_DEBUG", false)
	serverConfig.BodyLimit = "100M"

	// Service configuration
	serviceURL := common.GetEnv("GRAPHDB_SERVICE_URL", "")
	registryURL := common.GetEnv("GRAPHDB_REGISTRY_URL", "http://localhost:8096")
	apiKey := common.GetEnv("GRAPHDB_API_KEY", "")

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

	// Initialize state manager
	stateManager = statemanager.New(statemanager.Config{
		ServiceName:   "graphdbservice",
		MaxOperations: 100,
	})

	// Create Echo server with EVE http utilities
	e := evehttp.NewEchoServer(serverConfig)

	// Add security headers middleware
	e.Use(evehttp.SecurityHeadersMiddleware())

	// Initialize tracing (gracefully disabled if unavailable)
	if tracer := tracing.Init(tracing.InitConfig{
		ServiceID:        "graphdbservice",
		DisableIfMissing: true,
	}); tracer != nil {
		e.Use(tracer.Middleware())
	}

	// Register state endpoints
	apiGroup := e.Group("/v1/api")
	stateManager.RegisterRoutes(apiGroup)

	// API key middleware
	var apiKeyMiddleware echo.MiddlewareFunc
	if apiKey != "" {
		apiKeyMiddleware = evehttp.APIKeyMiddleware(apiKey)
		// Semantic action endpoint with API key protection (primary interface)
		apiGroup.POST("/semantic/action", handleSemanticAction, apiKeyMiddleware)
	} else {
		// Semantic action endpoint without protection (primary interface)
		apiGroup.POST("/semantic/action", handleSemanticAction)
	}

	// REST endpoints (convenience adapters that convert to semantic actions)
	if apiKey != "" {
		registerRESTEndpoints(apiGroup, apiKeyMiddleware)
	} else {
		registerRESTEndpoints(apiGroup, nil)
	}

	// Health check endpoint using EVE utilities (always public)
	e.GET("/health", evehttp.HealthCheckHandler("graphdb-semantic", "v1"))

	// Documentation endpoint
	e.GET("/v1/api/docs", evehttp.DocumentationHandler(evehttp.ServiceDocConfig{
		ServiceID:   "graphdb-semantic",
		ServiceName: "GraphDB Semantic Service",
		Description: "GraphDB repository and graph management via semantic actions",
		Version:     "v1",
		Port:        serverConfig.Port,
		Capabilities: []string{
			"graphdb-migration", "graphdb-create", "graphdb-delete",
			"graphdb-rename", "graphdb-import", "graphdb-export",
			"graph-migration", "graph-import", "graph-export",
			"graph-delete", "graph-rename", "state-tracking",
		},
		Endpoints: []evehttp.EndpointDoc{
			{
				Method:      "POST",
				Path:        "/v1/api/semantic/action",
				Description: "Execute semantic actions for GraphDB operations (primary interface)",
			},
			{
				Method:      "POST",
				Path:        "/v1/api/queries",
				Description: "Execute graph query (REST convenience - converts to SearchAction)",
			},
			{
				Method:      "POST",
				Path:        "/v1/api/nodes",
				Description: "Create node (REST convenience - converts to CreateAction)",
			},
			{
				Method:      "PUT",
				Path:        "/v1/api/nodes/:id",
				Description: "Update node (REST convenience - converts to UpdateAction)",
			},
			{
				Method:      "DELETE",
				Path:        "/v1/api/nodes/:id",
				Description: "Delete node (REST convenience - converts to DeleteAction)",
			},
			{
				Method:      "POST",
				Path:        "/v1/api/relationships",
				Description: "Create relationship (REST convenience - converts to CreateAction)",
			},
			{
				Method:      "GET",
				Path:        "/health",
				Description: "Health check endpoint",
			},
		},
	}))

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
		Version:     "v1",
		Hostname:    hostname,
		ServiceType: "graphdb",
		Capabilities: []string{
			"graphdb-migration", "graphdb-create", "graphdb-delete",
			"graphdb-rename", "graphdb-import", "graphdb-export",
			"graph-migration", "graph-import", "graph-export",
			"graph-delete", "graph-rename", "state-tracking",
		},
		Properties: map[string]interface{}{
			"semanticEndpoint": fmt.Sprintf("%s/v1/api/semantic/action", serviceURL),
			"healthEndpoint":   fmt.Sprintf("%s/health", serviceURL),
			"documentation":    fmt.Sprintf("%s/v1/api/docs", serviceURL),
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
