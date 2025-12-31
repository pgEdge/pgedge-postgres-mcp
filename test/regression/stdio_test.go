package regression

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ========================================================================
// TEST 09: MCP Server Stdio Mode Testing
// ========================================================================

// Test09_StdioMode tests MCP server in stdio mode with database connectivity
func (s *RegressionTestSuite) Test09_StdioMode() {
	s.T().Log("TEST 09: Testing MCP server in stdio mode")

	// Ensure packages and database are ready
	s.ensureMCPPackagesInstalled()

	// ====================================================================
	// STEP 0: Stop any running MCP server instances
	// ====================================================================
	s.logDetailed("Step 0: Stopping any running MCP server instances...")

	// Check if systemd service is running and stop it
	canUseSystemd := s.execMode == ModeContainerSystemd || s.execMode == ModeLocal
	if canUseSystemd {
		output, _, _ := s.execCmd(s.ctx, "systemctl is-active pgedge-postgres-mcp.service 2>/dev/null")
		if strings.Contains(output, "active") {
			s.T().Log("  Stopping systemd service...")
			s.execCmd(s.ctx, "systemctl stop pgedge-postgres-mcp.service")
			time.Sleep(2 * time.Second)
		}
	}

	// Kill any manually started processes
	s.execCmd(s.ctx, "pkill -f pgedge-postgres-mcp || true")
	time.Sleep(1 * time.Second)

	s.T().Log("  ✓ Any existing MCP server instances stopped")

	// ====================================================================
	// 1. Create stdio configuration file
	// ====================================================================
	s.logDetailed("Step 1: Creating stdio mode configuration...")

	// Create a temporary config file for stdio mode testing
	stdioConfig := `cat > /tmp/postgres-mcp-stdio-test.yaml << 'EOF'
databases:
  - host: localhost
    port: 5432
    name: mcp_server
    user: postgres
    password: postgres123
server:
  mode: stdio
EOF`
	output, exitCode, err := s.execCmd(s.ctx, stdioConfig)
	s.NoError(err, "Failed to create stdio config: %s", output)
	s.Equal(0, exitCode, "Create stdio config failed: %s", output)

	// Validate the config file was created with correct mode
	output, exitCode, err = s.execCmd(s.ctx, "grep 'mode: stdio' /tmp/postgres-mcp-stdio-test.yaml")
	s.NoError(err, "Failed to verify stdio mode in config: %s", output)
	s.Equal(0, exitCode, "Config should contain 'mode: stdio': %s", output)
	s.Contains(output, "mode: stdio", "Configuration must specify stdio mode")

	s.T().Log("  ✓ Stdio configuration created and validated (mode: stdio)")

	// ====================================================================
	// 2. Create test script to start MCP server in stdio mode
	// ====================================================================
	s.logDetailed("Step 2: Creating stdio test script...")

	// Create a script that will:
	// 1. Start the MCP server in stdio mode
	// 2. Send an initialize request
	// 3. Send a tools/list request to verify connectivity
	// 4. Kill after getting responses (stdio server waits forever for input)
	testScript := `cat > /tmp/test-stdio.sh << 'SCRIPT'
#!/bin/bash

# Timeout after 5 seconds total
timeout 5 /usr/bin/pgedge-postgres-mcp -config /tmp/postgres-mcp-stdio-test.yaml << 'INPUT' > /tmp/stdio-output.log 2>&1
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
INPUT

# Exit code 0 (normal exit after stdin closes) or 124 (timeout) are OK
exit_code=$?
if [ $exit_code -eq 0 ] || [ $exit_code -eq 124 ]; then
    exit 0
fi
exit $exit_code
SCRIPT`

	output, exitCode, err = s.execCmd(s.ctx, testScript)
	s.NoError(err, "Failed to create test script: %s", output)
	s.Equal(0, exitCode, "Create test script failed: %s", output)

	s.T().Log("  ✓ Stdio test script created")

	// ====================================================================
	// 3. Run the stdio test
	// ====================================================================
	s.logDetailed("Step 3: Starting MCP server in stdio mode...")

	output, exitCode, err = s.execCmd(s.ctx, "bash /tmp/test-stdio.sh")

	// Read the output log to verify responses
	logOutput, logExitCode, logErr := s.execCmd(s.ctx, "cat /tmp/stdio-output.log")
	if logErr == nil && logExitCode == 0 {
		s.T().Logf("Stdio output:\n%s", logOutput)
	}

	s.NoError(err, "Failed to run stdio test: %s\nLog: %s", output, logOutput)
	s.Equal(0, exitCode, "Stdio test script failed: %s\nLog: %s", output, logOutput)

	// ====================================================================
	// 4. Verify MCP server connected to database in stdio mode
	// ====================================================================
	s.logDetailed("Step 4: Verifying MCP server database connectivity in stdio mode...")

	// The output should contain JSON-RPC responses
	// Check for successful initialize response
	s.Contains(logOutput, `"jsonrpc":"2.0"`, "Expected JSON-RPC response in output")

	// Check for successful response (not an error)
	if strings.Contains(logOutput, `"error"`) && !strings.Contains(logOutput, `"result"`) {
		s.T().Logf("ERROR: Stdio mode returned error response:\n%s", logOutput)
		s.Fail("MCP server in stdio mode returned error response - likely database connection failed")
	}

	// Parse the responses to verify they're valid JSON-RPC
	lines := strings.Split(logOutput, "\n")
	foundInitialize := false
	foundToolsList := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			// Skip lines that aren't valid JSON
			continue
		}

		// Check if this is a JSON-RPC response
		if jsonrpc, ok := response["jsonrpc"].(string); ok && jsonrpc == "2.0" {
			// Check the ID to determine which response this is
			if id, ok := response["id"].(float64); ok {
				if int(id) == 1 {
					// Initialize response
					if result, ok := response["result"].(map[string]interface{}); ok {
						if serverInfo, ok := result["serverInfo"].(map[string]interface{}); ok {
							s.T().Logf("  ✓ Initialize response received: %s", serverInfo["name"])
							foundInitialize = true
						}
					}
				} else if int(id) == 2 {
					// Tools list response
					if result, ok := response["result"].(map[string]interface{}); ok {
						if tools, ok := result["tools"].([]interface{}); ok {
							s.T().Logf("  ✓ Tools list response received: %d tools available", len(tools))
							foundToolsList = true

							// Verify we have database-related tools (proves DB connectivity)
							toolNames := make([]string, 0, len(tools))
							for _, tool := range tools {
								if toolMap, ok := tool.(map[string]interface{}); ok {
									if name, ok := toolMap["name"].(string); ok {
										toolNames = append(toolNames, name)
									}
								}
							}

							// Check for key database tools - these only appear if MCP server connected to DB
							hasQueryTool := false
							hasSchemaInfo := false
							for _, name := range toolNames {
								if name == "query_database" {
									hasQueryTool = true
								}
								if name == "get_schema_info" {
									hasSchemaInfo = true
								}
							}

							s.True(hasQueryTool, "Expected 'query_database' tool - MCP server failed to connect to database")
							s.True(hasSchemaInfo, "Expected 'get_schema_info' tool - MCP server failed to connect to database")

							if hasQueryTool && hasSchemaInfo {
								s.T().Log("  ✓ MCP server successfully connected to database in stdio mode")
								s.T().Log("    (database tools available: query_database, get_schema_info)")
							}
						}
					}
				}
			}
		}
	}

	s.True(foundInitialize, "Expected to find initialize response")
	s.True(foundToolsList, "Expected to find tools/list response")

	// ====================================================================
	// 5. Execute actual database query to confirm MCP server DB connection
	// ====================================================================
	s.logDetailed("Step 5: Executing database query to confirm MCP server connected to DB...")

	// Create a more comprehensive test that actually queries the database
	queryTestScript := `cat > /tmp/test-stdio-query.sh << 'SCRIPT'
#!/bin/bash

# Timeout after 6 seconds total
timeout 6 /usr/bin/pgedge-postgres-mcp -config /tmp/postgres-mcp-stdio-test.yaml << 'INPUT' > /tmp/stdio-query-output.log 2>&1
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_schema_info","arguments":{}}}
INPUT

# Exit code 0 (normal exit after stdin closes) or 124 (timeout) are OK
exit_code=$?
if [ $exit_code -eq 0 ] || [ $exit_code -eq 124 ]; then
    exit 0
fi
exit $exit_code
SCRIPT`

	output, exitCode, err = s.execCmd(s.ctx, queryTestScript)
	s.NoError(err, "Failed to create query test script: %s", output)
	s.Equal(0, exitCode, "Create query test script failed: %s", output)

	// Give the previous process time to fully exit
	time.Sleep(2 * time.Second)

	// Run the query test
	output, exitCode, err = s.execCmd(s.ctx, "bash /tmp/test-stdio-query.sh")

	queryLogOutput, logExitCode, logErr := s.execCmd(s.ctx, "cat /tmp/stdio-query-output.log")
	if logErr == nil && logExitCode == 0 {
		s.T().Logf("Query test output:\n%s", queryLogOutput)
	}

	s.NoError(err, "Failed to run query test: %s\nLog: %s", output, queryLogOutput)
	s.Equal(0, exitCode, "Query test failed: %s\nLog: %s", output, queryLogOutput)

	// Verify we got schema information back - this proves MCP server connected to database
	foundSchemaResponse := false
	queryLines := strings.Split(queryLogOutput, "\n")
	for _, line := range queryLines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue
		}

		if id, ok := response["id"].(float64); ok && int(id) == 3 {
			// This is the get_schema_info response
			if result, ok := response["result"].(map[string]interface{}); ok {
				if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
					s.T().Log("  ✓ MCP server executed database query successfully")
					foundSchemaResponse = true

					// Log first content item to verify it has actual database schema data
					if firstContent, ok := content[0].(map[string]interface{}); ok {
						if text, ok := firstContent["text"].(string); ok && len(text) > 0 {
							s.T().Logf("  ✓ Retrieved schema data from database (%d bytes)", len(text))
							s.T().Log("  ✓ CONFIRMED: MCP server successfully connected to database in stdio mode")
						}
					}
				}
			}
		}
	}

	s.True(foundSchemaResponse, "Expected schema info response - MCP server failed to query database")

	// ====================================================================
	// 6. Stop the MCP server and verify it stopped
	// ====================================================================
	s.logDetailed("Step 6: Stopping MCP server after testing...")

	// Give the stdio servers time to exit after timeout kills them
	// The test scripts use timeout 5 and timeout 6, so wait longer than that
	// to ensure all timeout processes and their children have completed
	time.Sleep(8 * time.Second)

	// Kill any remaining timeout wrappers and MCP server processes
	// The timeout command may keep the process tree alive even after stdin closes
	// Use very specific pattern to avoid matching dnf/yum install processes
	s.execCmd(s.ctx, "pkill -9 -f 'timeout.*stdio-test.yaml' || true")
	s.execCmd(s.ctx, "pkill -9 -f '/usr/bin/pgedge-postgres-mcp.*stdio-test.yaml' || true")
	time.Sleep(2 * time.Second)

	// Verify all processes are gone using specific pattern that won't match DNF
	for attempt := 1; attempt <= 5; attempt++ {
		output, exitCode, err = s.execCmd(s.ctx, "pgrep -f '/usr/bin/pgedge-postgres-mcp.*stdio-test.yaml' || echo 'no-process'")
		if strings.Contains(output, "no-process") {
			break
		}

		// If processes still exist, show details and force kill them
		pids := strings.TrimSpace(output)
		if pids != "" && pids != "no-process" {
			// Show what these processes actually are
			pidList := strings.Fields(strings.ReplaceAll(pids, "\n", " "))
			if len(pidList) > 0 {
				s.T().Logf("  ⚠ Found %d lingering process(es): %v", len(pidList), pidList)
				// Show process details for debugging
				for _, pid := range pidList {
					psOut, _, _ := s.execCmd(s.ctx, "ps -fp "+pid+" 2>/dev/null || echo 'process-gone'")
					if !strings.Contains(psOut, "process-gone") {
						s.T().Logf("    Process %s: %s", pid, strings.TrimSpace(psOut))
					}
				}
			}

			// Kill all PIDs at once
			allPids := strings.Join(pidList, " ")
			s.T().Logf("  ⚠ Force killing PIDs: %s", allPids)
			s.execCmd(s.ctx, "sudo kill -9 "+allPids+" 2>/dev/null || kill -9 "+allPids+" 2>/dev/null || true")
		}
		time.Sleep(2 * time.Second)
	}

	// Final verification - no stdio mode processes should be running
	output, exitCode, err = s.execCmd(s.ctx, "pgrep -f '/usr/bin/pgedge-postgres-mcp.*stdio-test.yaml' || echo 'no-process'")
	if !strings.Contains(output, "no-process") {
		// Show detailed info about remaining processes before failing
		s.T().Logf("  ✗ ERROR: Processes still running after cleanup attempts!")
		s.execCmd(s.ctx, "ps aux | grep '[p]gedge-postgres-mcp.*stdio' || true")
	}
	s.Contains(output, "no-process", "MCP server stdio processes should be stopped")

	s.T().Log("  ✓ MCP server stopped and verified")

	// ====================================================================
	// 7. Cleanup test files
	// ====================================================================
	s.logDetailed("Step 7: Cleaning up test files...")

	s.execCmd(s.ctx, "rm -f /tmp/postgres-mcp-stdio-test.yaml")
	s.execCmd(s.ctx, "rm -f /tmp/test-stdio.sh")
	s.execCmd(s.ctx, "rm -f /tmp/test-stdio-query.sh")
	s.execCmd(s.ctx, "rm -f /tmp/stdio-output.log")
	s.execCmd(s.ctx, "rm -f /tmp/stdio-query-output.log")

	s.T().Log("  ✓ Test files cleaned up")

	s.T().Log("✓ MCP server stdio mode tests completed successfully")
	s.T().Log("  • Pre-check: Stopped any running MCP server instances")
	s.T().Log("  • Configuration: Validated stdio mode config (mode: stdio)")
	s.T().Log("  • Stdio mode: Started and responded to JSON-RPC requests")
	s.T().Log(fmt.Sprintf("  • Database connection: MCP server connected to PostgreSQL %s", s.pgVersion))
	s.T().Log("  • Database tools: Verified query_database and get_schema_info available")
	s.T().Log("  • Database query: Successfully retrieved schema data from database")
	s.T().Log("  • Shutdown: Server stopped cleanly after testing")
}
