package handler

import (
	"github.com/c-wind/mist-docs/internal/mcp"
	"github.com/gin-gonic/gin"
)

// MCPServe handles POST /api/mcp — the Model Context Protocol endpoint AI
// agents use to search and read this team's knowledge base. See internal/mcp
// for the protocol implementation; the adapter below binds the shared store.
func MCPServe(c *gin.Context) {
	mcpHandler.Serve(c)
}

var mcpHandler = mcp.New(mcpStore{})

// ensure mcp import stays used even if MCPServe body changes.
var _ = mcp.ServerName
