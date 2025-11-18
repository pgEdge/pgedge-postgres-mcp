/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Server
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import express from 'express';
import session from 'express-session';
import { readFileSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Load configuration
const configPath = process.env.CONFIG_FILE || join(__dirname, 'config.json');
const config = JSON.parse(readFileSync(configPath, 'utf-8'));

const app = express();
const PORT = process.env.PORT || config.server?.port || 3001;

// Middleware
app.use(express.json());
app.use(
  session({
    secret: config.session.secret,
    resave: false,
    saveUninitialized: false,
    cookie: {
      maxAge: config.session.maxAge,
      httpOnly: true,
      secure: process.env.NODE_ENV === 'production',
    },
  })
);

// Serve static files in production
if (process.env.NODE_ENV === 'production') {
  app.use(express.static(join(__dirname, 'dist')));
}

// Authentication middleware
const requireAuth = (req, res, next) => {
  if (!req.session.user) {
    return res.status(401).json({ error: 'Not authenticated' });
  }
  next();
};

// Helper function to call MCP server
async function callMCPServer(method, params, token = null) {
  const headers = {
    'Content-Type': 'application/json',
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const requestBody = {
    jsonrpc: '2.0',
    id: Date.now(),
    method,
    params: params || {},
  };

  console.log('MCP Request:', JSON.stringify(requestBody, null, 2));
  console.log('MCP Headers:', JSON.stringify(headers, null, 2));
  if (token) {
    console.log('Using session token:', token.substring(0, 20) + '...');
  } else {
    console.log('No token provided');
  }

  const response = await fetch(config.mcpServer.url, {
    method: 'POST',
    headers,
    body: JSON.stringify(requestBody),
  });

  console.log('MCP Response status:', response.status, response.statusText);

  if (!response.ok) {
    const errorText = await response.text();
    console.error('MCP Response error:', errorText);
    throw new Error(`MCP server error: ${response.statusText}`);
  }

  const data = await response.json();
  console.log('MCP Response data:', JSON.stringify(data, null, 2));

  if (data.error) {
    console.error('MCP Error response:', data.error);
    // Prefer error.data if available (contains actual error message), otherwise use error.message
    const errorMessage = data.error.data || data.error.message || 'MCP server error';
    throw new Error(errorMessage);
  }

  return data.result;
}

// API Routes

// Login endpoint
app.post('/api/login', async (req, res) => {
  try {
    const { username, password } = req.body;

    if (!username || !password) {
      return res.status(400).json({ message: 'Username and password are required' });
    }

    // Call MCP server's authenticate_user tool
    const result = await callMCPServer('tools/call', {
      name: 'authenticate_user',
      arguments: {
        username,
        password,
      },
    });

    // Check if authentication was successful
    if (!result.content || result.content.length === 0) {
      return res.status(401).json({ message: 'Invalid credentials' });
    }

    // Parse the result
    const content = result.content[0];
    let authData;

    if (content.type === 'text') {
      try {
        authData = JSON.parse(content.text);
      } catch (e) {
        // If it's not JSON, check if it's a success message
        if (content.text.includes('Authentication successful')) {
          authData = { success: true, username };
        } else {
          return res.status(401).json({ message: 'Invalid credentials' });
        }
      }
    }

    if (!authData || !authData.success) {
      return res.status(401).json({ message: 'Invalid credentials' });
    }

    // Store session token from authenticate_user
    const sessionToken = authData.session_token;
    if (!sessionToken) {
      return res.status(500).json({ message: 'No session token received from MCP server' });
    }

    // Store user and token in session
    req.session.user = {
      username: authData.username || username,
    };
    req.session.mcpToken = sessionToken;

    res.json({
      user: req.session.user,
    });
  } catch (error) {
    console.error('Login error:', error);
    res.status(500).json({ message: error.message || 'Login failed' });
  }
});

// Logout endpoint
app.post('/api/logout', (req, res) => {
  req.session.destroy((err) => {
    if (err) {
      return res.status(500).json({ error: 'Logout failed' });
    }
    res.json({ success: true });
  });
});

// Session check endpoint
app.get('/api/session', (req, res) => {
  if (req.session.user) {
    res.json({
      authenticated: true,
      user: req.session.user,
    });
  } else {
    res.json({
      authenticated: false,
    });
  }
});

// Get system info from MCP server
app.get('/api/mcp/system-info', requireAuth, async (req, res) => {
  try {
    const result = await callMCPServer(
      'resources/read',
      {
        uri: 'pg://system_info',
      },
      req.session.mcpToken
    );

    // Parse the system info from the result
    if (!result.contents || result.contents.length === 0) {
      return res.status(404).json({ error: 'System info not found' });
    }

    const content = result.contents[0];
    let systemInfo;

    // Check mimeType at result level (not content level)
    if (result.mimeType === 'application/json') {
      systemInfo = JSON.parse(content.text);
    } else {
      systemInfo = content.text;
    }

    res.json(systemInfo);
  } catch (error) {
    console.error('System info error:', error);
    res.status(500).json({ error: error.message || 'Failed to fetch system info' });
  }
});

// Serve React app in production
if (process.env.NODE_ENV === 'production') {
  app.get('*', (req, res) => {
    res.sendFile(join(__dirname, 'dist', 'index.html'));
  });
}

// Start server
app.listen(PORT, () => {
  console.log(`pgEdge MCP Client server running on port ${PORT}`);
  console.log(`Environment: ${process.env.NODE_ENV || 'development'}`);
  console.log(`MCP Server: ${config.mcpServer.url}`);
});
