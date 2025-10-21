package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/k6-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// toolMiddleware wraps a ToolHandler to add correlation ID, logging and recovery.
type toolMiddleware struct {
	name string
	next ToolHandler
}

func (m toolMiddleware) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Correlation and timing
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	start := time.Now()

	// Extract arguments for logging
	args := request.GetArguments()
	logging.RequestStart(ctx, m.name, args)

	// Panic safety
	defer func() {
		if rec := recover(); rec != nil {
			logging.RequestEnd(ctx, m.name, false, time.Since(start), fmt.Errorf("panic: %v", rec))
		}
	}()

	res, err := m.next.Handle(ctx, request)
	logging.RequestEnd(ctx, m.name, err == nil, time.Since(start), err)
	return res, err
}

// WithToolMiddleware decorates a ToolHandler with centralized boilerplate.
func WithToolMiddleware(name string, h ToolHandler) ToolHandler {
	return toolMiddleware{name: name, next: h}
}

// promptMiddleware wraps a PromptHandler to add correlation ID, logging and recovery.
type promptMiddleware struct {
	name string
	next PromptHandler
}

func (m promptMiddleware) Handle(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	start := time.Now()

	// Normalize arguments to map[string]interface{} for logging
	args := map[string]interface{}{}
	for k, v := range request.Params.Arguments {
		args[k] = v
	}
	logging.RequestStart(ctx, m.name, args)

	defer func() {
		if rec := recover(); rec != nil {
			logging.RequestEnd(ctx, m.name, false, time.Since(start), fmt.Errorf("panic: %v", rec))
		}
	}()

	res, err := m.next.Handle(ctx, request)
	logging.RequestEnd(ctx, m.name, err == nil, time.Since(start), err)
	return res, err
}

// WithPromptMiddleware decorates a PromptHandler with centralized boilerplate.
func WithPromptMiddleware(name string, h PromptHandler) PromptHandler {
	return promptMiddleware{name: name, next: h}
}
