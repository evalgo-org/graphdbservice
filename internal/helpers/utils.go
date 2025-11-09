// Package helpers provides utility functions for GraphDB operations.
package helpers

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// DebugMode controls whether detailed debug logging is enabled
var DebugMode bool = false

// DebugLog logs a message only if debug mode is enabled
func DebugLog(format string, args ...interface{}) {
	if DebugMode {
		fmt.Printf(DebugPrefix+format+"\n", args...)
	}
}

// DebugLogHTTP logs HTTP-related debug messages only if debug mode is enabled
func DebugLogHTTP(format string, args ...interface{}) {
	if DebugMode {
		fmt.Printf(DebugHTTPPrefix+format+"\n", args...)
	}
}

// NormalizeURL removes trailing slashes from URLs to prevent double-slash issues
func NormalizeURL(urlStr string) string {
	return strings.TrimRight(urlStr, "/")
}

// MD5Hash generates an MD5 hash of the given text string
func MD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// GetFileNames extracts filenames from a slice of multipart file headers
func GetFileNames(fileHeaders []*multipart.FileHeader) []string {
	names := make([]string, len(fileHeaders))
	for i, fh := range fileHeaders {
		names[i] = fh.Filename
	}
	return names
}

// URL2ServiceRobust parses a URL string and extracts the hostname.
// Adds scheme if missing to help url.Parse work correctly.
func URL2ServiceRobust(urlStr string) (string, error) {
	// Add scheme if missing
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	return parsedURL.Hostname(), nil
}

// GetFileType determines the RDF serialization format based on file extension
func GetFileType(filename string) string {
	filename = strings.ToLower(filename)

	switch {
	case strings.HasSuffix(filename, ExtBRF):
		return FormatBinaryRDF
	case strings.HasSuffix(filename, ExtRDF) || strings.HasSuffix(filename, ExtXML):
		return FormatRDFXML
	case strings.HasSuffix(filename, ExtTTL):
		return FormatTurtle
	case strings.HasSuffix(filename, ExtNTrips):
		return FormatNTriples
	case strings.HasSuffix(filename, ExtN3):
		return FormatN3
	case strings.HasSuffix(filename, ExtJSONLD) || strings.HasSuffix(filename, ExtJSON):
		return FormatJSONLD
	case strings.HasSuffix(filename, ExtTriG):
		return FormatTriG
	case strings.HasSuffix(filename, ExtNQuads):
		return FormatNQuads
	default:
		return FormatUnknown
	}
}

// DebugHTTPTransport wraps an http.RoundTripper to log request/response details
type DebugHTTPTransport struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper interface with debugging
func (d *DebugHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	DebugLogHTTP("%s %s", req.Method, req.URL.String())

	// Execute the request
	resp, err := d.Transport.RoundTrip(req)
	if err != nil {
		DebugLogHTTP("Request failed: %v", err)
		return resp, err
	}

	// Log response status
	DebugLogHTTP("Response Status: %d %s", resp.StatusCode, resp.Status)

	// Read and log the response body only for errors if debug mode is enabled
	if DebugMode && resp.StatusCode >= 400 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // Ignore error - body already read

		if readErr != nil {
			DebugLogHTTP("Failed to read error response body: %v", readErr)
		} else {
			DebugLogHTTP("===== ERROR RESPONSE BODY (Status %d) =====", resp.StatusCode)
			fmt.Printf("%s\n", string(bodyBytes))
			DebugLogHTTP("===== END ERROR RESPONSE BODY =====")

			// Restore the body for the caller
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	return resp, err
}

// EnableHTTPDebugLogging wraps the HTTP client with debug logging
func EnableHTTPDebugLogging(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}

	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}

	client.Transport = &DebugHTTPTransport{
		Transport: client.Transport,
	}

	return client
}

// UpdateRepositoryNameInConfig updates repository name references in a GraphDB TTL configuration file
func UpdateRepositoryNameInConfig(configFile, oldName, newName string) error {
	// Read the configuration file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Convert to string for processing
	configContent := string(content)

	// Replace repository ID references in the TTL file
	replacements := map[string]string{
		fmt.Sprintf(`rep:repositoryID "%s"`, oldName):                         fmt.Sprintf(`rep:repositoryID "%s"`, newName),
		fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, oldName): fmt.Sprintf(`<http://www.openrdf.org/config/repository#%s>`, newName),
		fmt.Sprintf(`repo:%s`, oldName):                                       fmt.Sprintf(`repo:%s`, newName),
	}

	// Apply replacements
	for old, new := range replacements {
		configContent = strings.ReplaceAll(configContent, old, new)
	}

	// Handle the repository node declaration if it exists
	basePattern := fmt.Sprintf(`@base <http://www.openrdf.org/config/repository#%s>`, oldName)
	newBasePattern := fmt.Sprintf(`@base <http://www.openrdf.org/config/repository#%s>`, newName)
	configContent = strings.ReplaceAll(configContent, basePattern, newBasePattern)

	// Write the updated content back to the file
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated config file: %w", err)
	}

	return nil
}

// GetGraphTripleCounts retrieves the triple counts for two graphs in a GraphDB repository.
// Returns -1 if counts are not available.
func GetGraphTripleCounts(url, username, password, repo, oldGraph, newGraph string) (int, int) {
	// Simplified implementation - returns -1 to indicate counts are not available
	return -1, -1
}
