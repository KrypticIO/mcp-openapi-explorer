package tools

import (
	"context"
	"fmt"

	"github.com/krypticio/mcp-openapi-explorer/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterGetAPIInfo registers the get_api_info tool
func RegisterGetAPIInfo(mcpServer *mcpserver.MCPServer, handler *server.MCPHandler, logger server.LoggerInterface) {
	// Create the get_api_info tool
	// This is the main tool that provides context about API endpoints based on user queries
	openAPITool := mcp.NewTool(
		"get_api_info",
		mcp.WithDescription("Get comprehensive information about API endpoints from loaded OpenAPI specifications"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Query about API endpoints (e.g. 'How do I create a new user?', 'What endpoints are available for pet management?')"),
		),
	)

	// Register the get_api_info tool
	mcpServer.AddTool(openAPITool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, ok := req.Params.Arguments["query"].(string)
		if !ok {
			return mcp.NewToolResultError("query parameter must be a string"), nil
		}

		// Use our handler to process the query and generate context about API endpoints
		mcpReq := &server.MCPRequest{Query: query}
		resp, err := handler.HandleMCPRequest(ctx, mcpReq)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error processing query: %v", err)), nil
		}

		// Return the generated context to the LLM
		return mcp.NewToolResultText(resp.Context), nil
	})
}
