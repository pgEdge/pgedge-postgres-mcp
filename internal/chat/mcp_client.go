/*-------------------------------------------------------------------------
 *
 * MCP Client for Chat Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package chat

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os/exec"
    "sync"

    "pgedge-postgres-mcp/internal/mcp"
)

// MCPClient provides a unified interface for communicating with MCP servers
// via both stdio and HTTP modes
type MCPClient interface {
    // Initialize establishes connection and performs handshake
    Initialize(ctx context.Context) error

    // ListTools returns available tools from the server
    ListTools(ctx context.Context) ([]mcp.Tool, error)

    // ListResources returns available resources from the server
    ListResources(ctx context.Context) ([]mcp.Resource, error)

    // CallTool executes a tool with the given arguments
    CallTool(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error)

    // ReadResource reads a resource by URI
    ReadResource(ctx context.Context, uri string) (mcp.ResourceContent, error)

    // Close cleans up resources
    Close() error
}

// stdioClient implements MCPClient for stdio communication
type stdioClient struct {
    cmd       *exec.Cmd
    stdin     io.WriteCloser
    stdout    io.ReadCloser
    scanner   *bufio.Scanner
    requestID int
    mu        sync.Mutex
}

// NewStdioClient creates a new stdio-based MCP client
func NewStdioClient(serverPath string) (MCPClient, error) {
    cmd := exec.Command(serverPath)

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
    }

    // Start the subprocess
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start MCP server: %w", err)
    }

    scanner := bufio.NewScanner(stdout)
    scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max buffer

    return &stdioClient{
        cmd:       cmd,
        stdin:     stdin,
        stdout:    stdout,
        scanner:   scanner,
        requestID: 0,
    }, nil
}

func (c *stdioClient) Initialize(ctx context.Context) error {
    params := mcp.InitializeParams{
        ProtocolVersion: mcp.ProtocolVersion,
        Capabilities:    map[string]interface{}{},
        ClientInfo: mcp.ClientInfo{
            Name:    "pgedge-postgres-mcp-chat",
            Version: "1.0.0-alpha1",
        },
    }

    var result mcp.InitializeResult
    if err := c.sendRequest(ctx, "initialize", params, &result); err != nil {
        return fmt.Errorf("initialize failed: %w", err)
    }

    // Send initialized notification
    notification := mcp.JSONRPCRequest{
        JSONRPC: "2.0",
        Method:  "notifications/initialized",
        Params:  map[string]interface{}{},
    }

    data, err := json.Marshal(notification)
    if err != nil {
        return fmt.Errorf("failed to marshal notification: %w", err)
    }

    if _, err := c.stdin.Write(append(data, '\n')); err != nil {
        return fmt.Errorf("failed to send notification: %w", err)
    }

    return nil
}

func (c *stdioClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
    var result mcp.ToolsListResult
    if err := c.sendRequest(ctx, "tools/list", nil, &result); err != nil {
        return nil, err
    }
    return result.Tools, nil
}

func (c *stdioClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
    var result mcp.ResourcesListResult
    if err := c.sendRequest(ctx, "resources/list", nil, &result); err != nil {
        return nil, err
    }
    return result.Resources, nil
}

func (c *stdioClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error) {
    params := mcp.ToolCallParams{
        Name:      name,
        Arguments: args,
    }

    var result mcp.ToolResponse
    if err := c.sendRequest(ctx, "tools/call", params, &result); err != nil {
        return mcp.ToolResponse{}, err
    }
    return result, nil
}

func (c *stdioClient) ReadResource(ctx context.Context, uri string) (mcp.ResourceContent, error) {
    params := mcp.ResourceReadParams{
        URI: uri,
    }

    var result mcp.ResourceContent
    if err := c.sendRequest(ctx, "resources/read", params, &result); err != nil {
        return mcp.ResourceContent{}, err
    }
    return result, nil
}

func (c *stdioClient) Close() error {
    if c.stdin != nil {
        c.stdin.Close()
    }
    if c.cmd != nil && c.cmd.Process != nil {
        _ = c.cmd.Process.Kill()   //nolint:errcheck // Best effort cleanup, errors not actionable
        _ = c.cmd.Wait()            //nolint:errcheck // Best effort cleanup, errors not actionable
    }
    return nil
}

func (c *stdioClient) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
    c.mu.Lock()
    c.requestID++
    id := c.requestID
    c.mu.Unlock()

    req := mcp.JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    reqData, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }

    // Send request
    if _, err := c.stdin.Write(append(reqData, '\n')); err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }

    // Read response
    if !c.scanner.Scan() {
        if err := c.scanner.Err(); err != nil {
            return fmt.Errorf("failed to read response: %w", err)
        }
        return fmt.Errorf("unexpected EOF")
    }

    var resp mcp.JSONRPCResponse
    if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
        return fmt.Errorf("failed to unmarshal response: %w", err)
    }

    if resp.Error != nil {
        return fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
    }

    // Marshal and unmarshal to convert to target type
    resultData, err := json.Marshal(resp.Result)
    if err != nil {
        return fmt.Errorf("failed to marshal result: %w", err)
    }

    if err := json.Unmarshal(resultData, result); err != nil {
        return fmt.Errorf("failed to unmarshal result: %w", err)
    }

    return nil
}

// httpClient implements MCPClient for HTTP communication
type httpClient struct {
    url       string
    token     string
    client    *http.Client
    requestID int
    mu        sync.Mutex
}

// NewHTTPClient creates a new HTTP-based MCP client
func NewHTTPClient(url, token string) MCPClient {
    return &httpClient{
        url:       url,
        token:     token,
        client:    &http.Client{},
        requestID: 0,
    }
}

func (c *httpClient) Initialize(ctx context.Context) error {
    params := mcp.InitializeParams{
        ProtocolVersion: mcp.ProtocolVersion,
        Capabilities:    map[string]interface{}{},
        ClientInfo: mcp.ClientInfo{
            Name:    "pgedge-postgres-mcp-chat",
            Version: "1.0.0-alpha1",
        },
    }

    var result mcp.InitializeResult
    if err := c.sendRequest(ctx, "initialize", params, &result); err != nil {
        return fmt.Errorf("initialize failed: %w", err)
    }

    return nil
}

func (c *httpClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
    var result mcp.ToolsListResult
    if err := c.sendRequest(ctx, "tools/list", nil, &result); err != nil {
        return nil, err
    }
    return result.Tools, nil
}

func (c *httpClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
    var result mcp.ResourcesListResult
    if err := c.sendRequest(ctx, "resources/list", nil, &result); err != nil {
        return nil, err
    }
    return result.Resources, nil
}

func (c *httpClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (mcp.ToolResponse, error) {
    params := mcp.ToolCallParams{
        Name:      name,
        Arguments: args,
    }

    var result mcp.ToolResponse
    if err := c.sendRequest(ctx, "tools/call", params, &result); err != nil {
        return mcp.ToolResponse{}, err
    }
    return result, nil
}

func (c *httpClient) ReadResource(ctx context.Context, uri string) (mcp.ResourceContent, error) {
    params := mcp.ResourceReadParams{
        URI: uri,
    }

    var result mcp.ResourceContent
    if err := c.sendRequest(ctx, "resources/read", params, &result); err != nil {
        return mcp.ResourceContent{}, err
    }
    return result, nil
}

func (c *httpClient) Close() error {
    return nil
}

func (c *httpClient) sendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
    c.mu.Lock()
    c.requestID++
    id := c.requestID
    c.mu.Unlock()

    req := mcp.JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    reqData, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(reqData))
    if err != nil {
        return fmt.Errorf("failed to create HTTP request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    if c.token != "" {
        httpReq.Header.Set("Authorization", "Bearer "+c.token)
    }

    resp, err := c.client.Do(httpReq)
    if err != nil {
        return fmt.Errorf("failed to send HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return fmt.Errorf("HTTP error %d (failed to read body: %w)", resp.StatusCode, err)
        }
        return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
    }

    var jsonResp mcp.JSONRPCResponse
    if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
        return fmt.Errorf("failed to decode response: %w", err)
    }

    if jsonResp.Error != nil {
        return fmt.Errorf("RPC error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
    }

    // Marshal and unmarshal to convert to target type
    resultData, err := json.Marshal(jsonResp.Result)
    if err != nil {
        return fmt.Errorf("failed to marshal result: %w", err)
    }

    if err := json.Unmarshal(resultData, result); err != nil {
        return fmt.Errorf("failed to unmarshal result: %w", err)
    }

    return nil
}
