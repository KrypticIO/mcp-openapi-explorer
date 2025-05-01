package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// SpecsDir is the directory where specs are stored
var SpecsDir string

// Logger is the global logger
var Logger LoggerInterface

// Config is the global configuration
var Config ConfigInterface

// LoggerInterface defines the required logging methods
type LoggerInterface interface {
	Debugw(msg string, keysAndValues ...interface{})
	Infow(msg string, keysAndValues ...interface{})
	Warnw(msg string, keysAndValues ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
	Fatalw(msg string, keysAndValues ...interface{})
}

// ConfigInterface defines the required config methods
type ConfigInterface interface {
	GetGitHubToken() string
	GetSpecs() []string
	ShouldUpdateConfig() bool
	HasSpec(url string) bool
	AddSpec(url string)
	RemoveSpec(url string)
	Save() error
}

// APISpec represents an OpenAPI specification
type APISpec struct {
	URL  string      `json:"url"`
	Spec *openapi3.T `json:"spec"`
}

// MCPHandler handles OpenAPI specs for the MCP server
type MCPHandler struct {
	specs map[string]*APISpec
}

// MCPRequest represents an MCP request
type MCPRequest struct {
	Query string `json:"query"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	Context string `json:"context"`
}

// NewHandler creates a new MCPHandler
func NewHandler(specsDir string, logger LoggerInterface, config ConfigInterface) *MCPHandler {
	SpecsDir = specsDir
	Logger = logger
	Config = config

	handler := &MCPHandler{
		specs: make(map[string]*APISpec),
	}

	// Load specs from disk
	if err := handler.loadSpecs(); err != nil {
		Logger.Warnw("Failed to load specs from disk", "error", err)
	}

	return handler
}

// LoadOpenAPISpec loads an OpenAPI specification from a URL
func (h *MCPHandler) LoadOpenAPISpec(ctx context.Context, specURL string) (*openapi3.T, error) {
	Logger.Debugw("Loading OpenAPI spec", "url", specURL)

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Handle GitHub URLs
	if strings.Contains(specURL, "github.com") && !strings.Contains(specURL, "raw.githubusercontent.com") {
		// Convert github.com URL to raw.githubusercontent.com
		specURL = strings.Replace(specURL, "github.com", "raw.githubusercontent.com", 1)
		specURL = strings.Replace(specURL, "/blob/", "/", 1)
		Logger.Debugw("Converted GitHub URL", "url", specURL)
	}

	parsedURL, err := url.Parse(specURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	Logger.Debugw("Parsed URL", "scheme", parsedURL.Scheme)

	var data []byte

	if parsedURL.Scheme == "file" || parsedURL.Scheme == "" {
		// Handle local file paths
		path := specURL
		if parsedURL.Scheme == "file" {
			path = strings.TrimPrefix(specURL, "file://")
		}
		Logger.Debugw("Loading from local file", "path", path)
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read local file: %w", err)
		}
	} else {
		// Handle HTTP/HTTPS URLs
		Logger.Debugw("Fetching spec from URL", "url", specURL)

		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add authentication header if this is a GitHub URL and a token is provided
		if strings.Contains(specURL, "githubusercontent.com") && Config.GetGitHubToken() != "" {
			req.Header.Add("Authorization", fmt.Sprintf("token %s", Config.GetGitHubToken()))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec: %w", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				Logger.Errorw("Failed to close response body", "error", err)
			}
		}(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch spec: HTTP %d", resp.StatusCode)
		}

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		Logger.Debugw("Received data", "bytes", len(data))
	}

	// Determine if this is YAML or JSON
	var spec *openapi3.T

	// Try to detect if it's YAML or JSON
	if isJSON(data) {
		Logger.Debugw("Parsing as JSON")
		spec, err = loader.LoadFromData(data)
	} else {
		Logger.Debugw("Parsing as YAML")
		// Convert YAML to JSON first
		var jsonData map[string]interface{}
		if err := yaml.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Convert the map to JSON
		jsonBytes, err := json.Marshal(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}

		// Now load from the JSON data
		spec, _ = loader.LoadFromData(jsonBytes)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	Logger.Debugw("Successfully loaded OpenAPI spec", "title", spec.Info.Title, "version", spec.Info.Version)
	return spec, nil
}

// isJSON tries to determine if the data is JSON by looking for characteristic JSON patterns
func isJSON(data []byte) bool {
	// Trim leading whitespace
	for i, b := range data {
		if !isWhitespace(b) {
			data = data[i:]
			break
		}
	}

	// Check if it starts with a JSON object or array
	return len(data) > 0 && (data[0] == '{' || data[0] == '[')
}

// isWhitespace returns true if the byte is a whitespace character
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// HandleMCPRequest handles MCP requests
func (h *MCPHandler) HandleMCPRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error) {
	Logger.Debugw("Received MCP request", "query", req.Query)

	// If no specs are loaded, return a helpful message
	if len(h.specs) == 0 {
		return &MCPResponse{
			Context: "No API specifications have been loaded. Please load an OpenAPI spec first.",
		}, nil
	}

	// Generate comprehensive apiContext about all loaded API specs
	var apiContext strings.Builder

	// Provide an overview of available specs
	apiContext.WriteString("# Available API Specifications\n\n")
	for _, spec := range h.specs {
		apiContext.WriteString(fmt.Sprintf("## %s (Version: %s)\n", spec.Spec.Info.Title, spec.Spec.Info.Version))
		if spec.Spec.Info.Description != "" {
			apiContext.WriteString(fmt.Sprintf("%s\n\n", spec.Spec.Info.Description))
		}
		apiContext.WriteString(fmt.Sprintf("Source: %s\n\n", spec.URL))
	}

	// Provide details about endpoints for each spec
	apiContext.WriteString("# API Endpoints\n\n")
	for _, spec := range h.specs {
		apiContext.WriteString(fmt.Sprintf("## %s Endpoints\n\n", spec.Spec.Info.Title))

		// Sort paths for consistent output
		paths := make([]string, 0, len(spec.Spec.Paths.Map()))
		for path := range spec.Spec.Paths.Map() {
			paths = append(paths, path)
		}
		// Simple alphabetical sort for now
		sort.Strings(paths)

		for _, path := range paths {
			pathItem := spec.Spec.Paths.Find(path)
			if pathItem == nil {
				continue
			}

			apiContext.WriteString(fmt.Sprintf("### Path: %s\n\n", path))

			for method, operation := range pathItem.Operations() {
				apiContext.WriteString(fmt.Sprintf("#### %s\n\n", strings.ToUpper(method)))

				if operation.Summary != "" {
					apiContext.WriteString(fmt.Sprintf("Summary: %s\n\n", operation.Summary))
				}

				if operation.Description != "" {
					apiContext.WriteString(fmt.Sprintf("Description: %s\n\n", operation.Description))
				}

				// Parameters
				if len(operation.Parameters) > 0 {
					apiContext.WriteString("Parameters:\n")
					for _, param := range operation.Parameters {
						required := ""
						if param.Value.Required {
							required = " (Required)"
						}
						apiContext.WriteString(fmt.Sprintf("- %s (%s)%s: %s\n",
							param.Value.Name,
							param.Value.In,
							required,
							param.Value.Description))
					}
					apiContext.WriteString("\n")
				}

				// Request Body
				if operation.RequestBody != nil && operation.RequestBody.Value != nil {
					apiContext.WriteString("Request Body:\n")
					if operation.RequestBody.Value.Description != "" {
						apiContext.WriteString(fmt.Sprintf("Description: %s\n", operation.RequestBody.Value.Description))
					}

					for contentType, mediaType := range operation.RequestBody.Value.Content {
						apiContext.WriteString(fmt.Sprintf("Content-Type: %s\n", contentType))
						if mediaType.Schema != nil {
							if mediaType.Schema.Ref != "" {
								apiContext.WriteString(fmt.Sprintf("Schema Reference: %s\n", mediaType.Schema.Ref))
							} else if mediaType.Schema.Value != nil {
								apiContext.WriteString(fmt.Sprintf("Schema Type: %s\n", mediaType.Schema.Value.Type))
							}
						}
					}
					apiContext.WriteString("\n")
				}

				// Responses
				if operation.Responses != nil {
					apiContext.WriteString("Responses:\n")
					for status, response := range operation.Responses.Map() {
						if response != nil && response.Value != nil {
							desc := "No description provided"
							if response.Value.Description != nil && *response.Value.Description != "" {
								desc = *response.Value.Description
							}
							apiContext.WriteString(fmt.Sprintf("- %s: %s\n", status, desc))
						}
					}
					apiContext.WriteString("\n")
				}
			}
		}
	}

	return &MCPResponse{
		Context: apiContext.String(),
	}, nil
}

// RegisterSpec registers a new OpenAPI specification via HTTP
func (h *MCPHandler) RegisterSpec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.Errorw("Failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	Logger.Infow("Registering new spec", "url", req.URL)
	spec, err := h.LoadOpenAPISpec(r.Context(), req.URL)
	if err != nil {
		Logger.Errorw("Failed to load OpenAPI spec", "error", err, "url", req.URL)
		http.Error(w, fmt.Sprintf("Failed to load spec: %v", err), http.StatusBadRequest)
		return
	}

	// Generate a unique ID for this spec
	specID := filepath.Base(req.URL)
	h.specs[specID] = &APISpec{
		URL:  req.URL,
		Spec: spec,
	}

	Logger.Infow("Successfully registered spec", "id", specID, "title", spec.Info.Title)
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id": specID,
	}); err != nil {
		Logger.Errorw("Failed to encode response", "error", err)
	}
}

// GetSpec gets a registered OpenAPI specification via HTTP
func (h *MCPHandler) GetSpec(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	specID := vars["id"]

	Logger.Debugw("Getting spec", "id", specID)
	spec, exists := h.specs[specID]
	if !exists {
		Logger.Warnw("Spec not found", "id", specID)
		http.Error(w, "Spec not found", http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(spec); err != nil {
		Logger.Errorw("Failed to encode api specs", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// ListEndpoints lists all endpoints in a registered OpenAPI specification via HTTP
func (h *MCPHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	specID := vars["id"]

	Logger.Debugw("Listing endpoints for spec", "id", specID)
	spec, exists := h.specs[specID]
	if !exists {
		Logger.Warnw("Spec not found", "id", specID)
		http.Error(w, "Spec not found", http.StatusNotFound)
		return
	}

	endpoints := make([]map[string]interface{}, 0)
	if spec.Spec.Paths != nil {
		Logger.Debugw("Found paths in spec", "count", len(spec.Spec.Paths.Map()))
		for path, pathItem := range spec.Spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				endpoint := map[string]interface{}{
					"path":        path,
					"method":      method,
					"summary":     operation.Summary,
					"description": operation.Description,
					"parameters":  operation.Parameters,
					"requestBody": operation.RequestBody,
					"responses":   operation.Responses,
				}
				endpoints = append(endpoints, endpoint)
				Logger.Debugw("Added endpoint", "method", method, "path", path)
			}
		}
	} else {
		Logger.Warnw("No paths found in spec")
	}

	Logger.Infow("Returning endpoints", "count", len(endpoints))
	if err := json.NewEncoder(w).Encode(endpoints); err != nil {
		Logger.Errorw("Failed to encode endpoints", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// SaveSpec saves a spec to a file
func (h *MCPHandler) SaveSpec(specID string, spec *APISpec) error {
	// Create specs directory if it doesn't exist
	if err := os.MkdirAll(SpecsDir, 0755); err != nil {
		return fmt.Errorf("failed to create specs directory: %w", err)
	}

	// Marshal the API spec to JSON
	specData, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	// Save the spec to a file
	specPath := filepath.Join(SpecsDir, specID+".json")
	if err := os.WriteFile(specPath, specData, 0644); err != nil {
		return fmt.Errorf("failed to write spec file: %w", err)
	}

	Logger.Debugw("Saved spec", "id", specID, "path", specPath)
	return nil
}

// LoadSpecs loads specs from files
func (h *MCPHandler) loadSpecs() error {
	// Create specs directory if it doesn't exist
	if err := os.MkdirAll(SpecsDir, 0755); err != nil {
		return fmt.Errorf("failed to create specs directory: %w", err)
	}

	// Get all JSON files in the specs directory
	files, err := filepath.Glob(filepath.Join(SpecsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list spec files: %w", err)
	}

	// Load each spec file
	for _, file := range files {
		specID := filepath.Base(file)
		specID = specID[:len(specID)-5] // Remove .json extension

		// Read the spec file
		specData, err := os.ReadFile(file)
		if err != nil {
			Logger.Warnw("Failed to read spec file", "file", file, "error", err)
			continue
		}

		// Unmarshal the spec
		var spec APISpec
		if err := json.Unmarshal(specData, &spec); err != nil {
			Logger.Warnw("Failed to unmarshal spec", "file", file, "error", err)
			continue
		}

		// Add the spec to the handler
		h.specs[specID] = &spec
		Logger.Debugw("Loaded spec", "id", specID, "file", file)
	}

	Logger.Infow("Loaded specs", "count", len(h.specs), "dir", SpecsDir)
	return nil
}

// DeleteSpec removes a spec from memory and deletes its file
func (h *MCPHandler) DeleteSpec(specID string) error {
	// Safety check - make sure we have a valid specID
	if specID == "" {
		return fmt.Errorf("empty spec ID provided")
	}

	// Check if the spec exists
	_, exists := h.specs[specID]
	if !exists {
		return fmt.Errorf("spec not found: %s", specID)
	}

	// Get a clean filename - avoid any potential path traversal issues
	sanitizedSpecID := filepath.Base(specID)
	specPath := filepath.Join(SpecsDir, sanitizedSpecID+".json")

	// Create a copy of the spec reference before deletion (for logging)
	specCopy := h.specs[specID]
	specTitle := "unknown"
	if specCopy != nil && specCopy.Spec != nil && specCopy.Spec.Info != nil {
		specTitle = specCopy.Spec.Info.Title
	}

	// Delete the spec file if it exists
	if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
		// Log the error but continue - we'll still remove from memory
		Logger.Warnw("Failed to delete spec file",
			"error", err,
			"specID", specID,
			"title", specTitle,
			"path", specPath)
	} else {
		Logger.Debugw("Deleted spec file",
			"specID", specID,
			"title", specTitle,
			"path", specPath)
	}

	// Remove from memory
	delete(h.specs, specID)
	Logger.Infow("Removed spec from memory",
		"specID", specID,
		"title", specTitle)

	return nil
}

// GetSpecs returns a copy of the loaded specs map
func (h *MCPHandler) GetSpecs() map[string]*APISpec {
	return h.specs
}

// AddSpec adds a spec to the handler
func (h *MCPHandler) AddSpec(specID string, spec *APISpec) {
	h.specs[specID] = spec
}

// loadOpenAPISpec is a wrapper for the public LoadOpenAPISpec method
func (h *MCPHandler) loadOpenAPISpec(ctx context.Context, specURL string) (*openapi3.T, error) {
	return h.LoadOpenAPISpec(ctx, specURL)
}
