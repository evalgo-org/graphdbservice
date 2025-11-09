// Package helpers provides utility functions and constants for GraphDB operations.
package helpers

// Multipart form field keys
const (
	MultipartConfigKeyFormat = "task_%d_config"
	MultipartFilesKeyFormat  = "task_%d_files"
)

// Temporary file prefixes
const (
	TempFileRepoImportPrefix   = "repo_import_"
	TempFileRepoCreatePrefix   = "repo_create_"
	TempFileGraphImportPrefix  = "graph_import_"
	TempFileGraphRenamePrefix  = "graph_rename_"
	TempFileRepoRenamePrefix   = "repo_rename_"
)

// File extensions
const (
	ExtBRF    = ".brf"
	ExtRDF    = ".rdf"
	ExtXML    = ".xml"
	ExtTTL    = ".ttl"
	ExtNTrips = ".nt"
	ExtN3     = ".n3"
	ExtJSONLD = ".jsonld"
	ExtJSON   = ".json"
	ExtTriG   = ".trig"
	ExtNQuads = ".nq"
)

// RDF format types
const (
	FormatBinaryRDF = "binary-rdf"
	FormatRDFXML    = "rdf-xml"
	FormatTurtle    = "turtle"
	FormatNTriples  = "n-triples"
	FormatN3        = "n3"
	FormatJSONLD    = "json-ld"
	FormatTriG      = "trig"
	FormatNQuads    = "n-quads"
	FormatUnknown   = "unknown"
)

// GraphDB response types
type GraphDBBinding map[string]interface{}

// Debug log messages
const (
	DebugPrefix    = "DEBUG: "
	DebugHTTPPrefix = "DEBUG HTTP: "
)
