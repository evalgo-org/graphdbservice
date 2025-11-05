package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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
	// Get configuration
	port, _ := cmd.Flags().GetInt("port")
	serviceURL, _ := cmd.Flags().GetString("service-url")
	registryURL, _ := cmd.Flags().GetString("registry-url")
	apiKey, _ := cmd.Flags().GetString("api-key")
	debug, _ := cmd.Flags().GetBool("debug")

	// Override with environment variables
	if envPort := os.Getenv("GRAPHDB_SERVICE_PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	}
	if envURL := os.Getenv("GRAPHDB_SERVICE_URL"); envURL != "" {
		serviceURL = envURL
	}
	if envRegistry := os.Getenv("REGISTRYSERVICE_API_URL"); envRegistry != "" {
		registryURL = envRegistry
	}
	if envKey := os.Getenv("API_KEY"); envKey != "" {
		apiKey = envKey
	}
	if os.Getenv("DEBUG") == "true" {
		debug = true
	}

	// Set debug mode globally
	debugMode = debug

	// Determine service URL
	if serviceURL == "" {
		hostname := os.Getenv("HOSTNAME")
		if hostname == "" {
			var err error
			hostname, err = os.Hostname()
			if err != nil {
				hostname = "localhost"
			}
		}
		serviceURL = fmt.Sprintf("http://%s:%d", hostname, port)
	}

	// Default registry URL
	if registryURL == "" {
		registryURL = "http://localhost:8096"
	}

	fmt.Println("=====================================")
	fmt.Println("GraphDB Semantic Service")
	fmt.Println("=====================================")
	fmt.Printf("Service URL: %s\n", serviceURL)
	fmt.Printf("Registry URL: %s\n", registryURL)
	fmt.Printf("Port: %d\n", port)
	if apiKey != "" {
		fmt.Println("API Key: *** (protected)")
	} else {
		fmt.Println("API Key: None (service is unprotected)")
	}
	fmt.Printf("Debug Mode: %v\n", debug)
	fmt.Println("=====================================")

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("100M"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.HEAD, echo.PUT, echo.PATCH, echo.POST, echo.DELETE},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "x-api-key"},
	}))

	// API key middleware (if configured)
	if apiKey != "" {
		apiKeyMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				key := c.Request().Header.Get("x-api-key")
				if key != apiKey {
					return echo.NewHTTPError(http.StatusUnauthorized, "Invalid or missing API key")
				}
				return next(c)
			}
		}
		// Semantic action endpoint with API key protection
		e.POST("/v1/api/semantic/action", handleSemanticAction, apiKeyMiddleware)
	} else {
		// Semantic action endpoint without protection
		e.POST("/v1/api/semantic/action", handleSemanticAction)
	}

	// Health check endpoint (always public)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "healthy",
			"service": "graphdb-semantic",
			"version": "2.0.0",
		})
	})

	// Start server in goroutine
	go func() {
		addr := fmt.Sprintf(":%d", port)
		fmt.Printf("\nStarting server on %s\n", addr)
		fmt.Printf("Semantic endpoint: POST %s/v1/api/semantic/action\n", serviceURL)
		fmt.Printf("Health endpoint: GET %s/health\n\n", serviceURL)

		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			fmt.Printf("FATAL: Failed to start server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Register with registry service
	ctx := context.Background()
	if err := registerWithRegistry(ctx, serviceURL, registryURL); err != nil {
		fmt.Printf("WARNING: Failed to register with registry: %v\n", err)
		fmt.Println("Service will continue without registry registration")
	} else {
		fmt.Println("Successfully registered with registry service")
		// Start heartbeat
		go startRegistryHeartbeat(ctx, serviceURL, registryURL)
	}

	fmt.Println("\nService is ready. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down service...")

	// Deregister from registry
	if err := deregisterFromRegistry(context.Background(), serviceURL, registryURL); err != nil {
		fmt.Printf("WARNING: Failed to deregister: %v\n", err)
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
	}

	fmt.Println("Service stopped")
}
