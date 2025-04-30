package cmd

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

// APISpec represents an OpenAPI specification
type APISpec struct {
	URL  string      `json:"url"`
	Spec *openapi3.T `json:"spec"`
}

// MCPHandler handles OpenAPI specs and MCP requests
type MCPHandler struct {
	specs map[string]*APISpec
}

// MCPRequest represents a request to the MCP server
type MCPRequest struct {
	Query string `json:"query"`
}

// MCPResponse represents a response from the MCP server
type MCPResponse struct {
	Context string `json:"context"`
}

// NewMCPHandler creates a new MCPHandler
func NewMCPHandler() *MCPHandler {
	handler := &MCPHandler{
		specs: make(map[string]*APISpec),
	}

	// Load specs from files
	if err := handler.loadSpecs(); err != nil {
		Logger.Warnw("Failed to load specs", "error", err)
	}

	return handler
}

// loadOpenAPISpec loads an OpenAPI specification from a URL
func (h *MCPHandler) loadOpenAPISpec(ctx context.Context, specURL string) (*openapi3.T, error) {
	Logger.Debugw("Loading OpenAPI spec", "url", specURL)

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Handle GitHub URLs (this is a simpler version, we have a more comprehensive version in serve.go)
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
		if strings.Contains(specURL, "githubusercontent.com") && Config.GitHub.Token != "" {
			req.Header.Add("Authorization", fmt.Sprintf("token %s", Config.GitHub.Token))
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch spec: %w", err)
		}
		defer resp.Body.Close()

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
		spec, err = loader.LoadFromData(jsonBytes)
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

// generateEndpointContext generates context information for an API endpoint
func (h *MCPHandler) generateEndpointContext(path string, method string, operation *openapi3.Operation) string {
	var context strings.Builder

	context.WriteString(fmt.Sprintf("Endpoint: %s %s\n", method, path))
	if operation.Summary != "" {
		context.WriteString(fmt.Sprintf("Summary: %s\n", operation.Summary))
	}
	if operation.Description != "" {
		context.WriteString(fmt.Sprintf("Description: %s\n", operation.Description))
	}

	// Parameters
	if len(operation.Parameters) > 0 {
		context.WriteString("\nParameters:\n")
		for _, param := range operation.Parameters {
			context.WriteString(fmt.Sprintf("- %s (%s): %s\n", param.Value.Name, param.Value.In, param.Value.Description))
			if param.Value.Required {
				context.WriteString("  Required: true\n")
			}
		}
	}

	// Request Body
	if operation.RequestBody != nil {
		context.WriteString("\nRequest Body:\n")
		if operation.RequestBody.Value.Description != "" {
			context.WriteString(fmt.Sprintf("Description: %s\n", operation.RequestBody.Value.Description))
		}
		for contentType, mediaType := range operation.RequestBody.Value.Content {
			context.WriteString(fmt.Sprintf("Content-Type: %s\n", contentType))
			if mediaType.Schema != nil {
				context.WriteString("Schema: ")
				if mediaType.Schema.Ref != "" {
					context.WriteString(fmt.Sprintf("Reference to %s\n", mediaType.Schema.Ref))
				} else if mediaType.Schema.Value != nil {
					context.WriteString(fmt.Sprintf("Type: %s\n", mediaType.Schema.Value.Type))
				}
			}
		}
	}

	// Responses
	if operation.Responses != nil {
		context.WriteString("\nResponses:\n")
		for status, response := range operation.Responses.Map() {
			if response != nil && response.Value != nil {
				desc := "No description provided"
				if response.Value.Description != nil && *response.Value.Description != "" {
					desc = *response.Value.Description
				}
				context.WriteString(fmt.Sprintf("- %s: %s\n", status, desc))
			}
		}
		context.WriteString("\n")
	}

	return context.String()
}

// handleMCPRequest handles MCP requests
func (h *MCPHandler) handleMCPRequest(ctx context.Context, req *MCPRequest) (*MCPResponse, error) {
	Logger.Debugw("Received MCP request", "query", req.Query)

	// If no specs are loaded, return a helpful message
	if len(h.specs) == 0 {
		return &MCPResponse{
			Context: "No API specifications have been loaded. Please load an OpenAPI spec first.",
		}, nil
	}

	// Generate comprehensive context about all loaded API specs
	var context strings.Builder

	// Provide an overview of available specs
	context.WriteString("# Available API Specifications\n\n")
	for _, spec := range h.specs {
		context.WriteString(fmt.Sprintf("## %s (Version: %s)\n", spec.Spec.Info.Title, spec.Spec.Info.Version))
		if spec.Spec.Info.Description != "" {
			context.WriteString(fmt.Sprintf("%s\n\n", spec.Spec.Info.Description))
		}
		context.WriteString(fmt.Sprintf("Source: %s\n\n", spec.URL))
	}

	// Provide details about endpoints for each spec
	context.WriteString("# API Endpoints\n\n")
	for _, spec := range h.specs {
		context.WriteString(fmt.Sprintf("## %s Endpoints\n\n", spec.Spec.Info.Title))

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

			context.WriteString(fmt.Sprintf("### Path: %s\n\n", path))

			for method, operation := range pathItem.Operations() {
				context.WriteString(fmt.Sprintf("#### %s\n\n", strings.ToUpper(method)))

				if operation.Summary != "" {
					context.WriteString(fmt.Sprintf("Summary: %s\n\n", operation.Summary))
				}

				if operation.Description != "" {
					context.WriteString(fmt.Sprintf("Description: %s\n\n", operation.Description))
				}

				// Parameters
				if len(operation.Parameters) > 0 {
					context.WriteString("Parameters:\n")
					for _, param := range operation.Parameters {
						required := ""
						if param.Value.Required {
							required = " (Required)"
						}
						context.WriteString(fmt.Sprintf("- %s (%s)%s: %s\n",
							param.Value.Name,
							param.Value.In,
							required,
							param.Value.Description))
					}
					context.WriteString("\n")
				}

				// Request Body
				if operation.RequestBody != nil && operation.RequestBody.Value != nil {
					context.WriteString("Request Body:\n")
					if operation.RequestBody.Value.Description != "" {
						context.WriteString(fmt.Sprintf("Description: %s\n", operation.RequestBody.Value.Description))
					}

					for contentType, mediaType := range operation.RequestBody.Value.Content {
						context.WriteString(fmt.Sprintf("Content-Type: %s\n", contentType))
						if mediaType.Schema != nil {
							if mediaType.Schema.Ref != "" {
								context.WriteString(fmt.Sprintf("Schema Reference: %s\n", mediaType.Schema.Ref))
							} else if mediaType.Schema.Value != nil {
								context.WriteString(fmt.Sprintf("Schema Type: %s\n", mediaType.Schema.Value.Type))
							}
						}
					}
					context.WriteString("\n")
				}

				// Responses
				if operation.Responses != nil {
					context.WriteString("Responses:\n")
					for status, response := range operation.Responses.Map() {
						if response != nil && response.Value != nil {
							desc := "No description provided"
							if response.Value.Description != nil && *response.Value.Description != "" {
								desc = *response.Value.Description
							}
							context.WriteString(fmt.Sprintf("- %s: %s\n", status, desc))
						}
					}
					context.WriteString("\n")
				}
			}
		}
	}

	return &MCPResponse{
		Context: context.String(),
	}, nil
}

// RegisterSpec registers a new OpenAPI specification
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
	spec, err := h.loadOpenAPISpec(r.Context(), req.URL)
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
	json.NewEncoder(w).Encode(map[string]string{
		"id": specID,
	})
}

// GetSpec gets a registered OpenAPI specification
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

	json.NewEncoder(w).Encode(spec)
}

// ListEndpoints lists all endpoints in a registered OpenAPI specification
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
		Logger.Warn("No paths found in spec")
	}

	Logger.Infow("Returning endpoints", "count", len(endpoints))
	json.NewEncoder(w).Encode(endpoints)
}

// Add this method to the MCPHandler struct
func (h *MCPHandler) saveSpec(specID string, spec *APISpec) error {
	// Create specs directory if it doesn't exist
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return fmt.Errorf("failed to create specs directory: %w", err)
	}

	// Marshal the API spec to JSON
	specData, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	// Save the spec to a file
	specPath := filepath.Join(specsDir, specID+".json")
	if err := os.WriteFile(specPath, specData, 0644); err != nil {
		return fmt.Errorf("failed to write spec file: %w", err)
	}

	Logger.Debugw("Saved spec", "id", specID, "path", specPath)
	return nil
}

// Add this method to the MCPHandler struct
func (h *MCPHandler) loadSpecs() error {
	// Create specs directory if it doesn't exist
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return fmt.Errorf("failed to create specs directory: %w", err)
	}

	// Get all JSON files in the specs directory
	files, err := filepath.Glob(filepath.Join(specsDir, "*.json"))
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

	Logger.Infow("Loaded specs", "count", len(h.specs), "dir", specsDir)
	return nil
}

// deleteSpec removes a spec from memory and deletes its file
func (h *MCPHandler) deleteSpec(specID string) error {
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
	specPath := filepath.Join(specsDir, sanitizedSpecID+".json")

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
