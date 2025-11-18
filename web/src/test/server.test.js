/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Express Server Tests
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import request from 'supertest';
import express from 'express';
import session from 'express-session';

// Mock config
const mockConfig = {
  mcpServer: {
    url: 'http://localhost:8080/mcp/v1',
    name: 'Test MCP Server',
  },
  session: {
    secret: 'test-secret',
    maxAge: 86400000,
  },
  server: {
    port: 3001,
  },
};

// Create a test app similar to the real server
function createTestApp() {
  const app = express();
  app.use(express.json());
  app.use(
    session({
      secret: mockConfig.session.secret,
      resave: false,
      saveUninitialized: false,
      cookie: {
        secure: false,
        httpOnly: true,
        maxAge: mockConfig.session.maxAge,
      },
    })
  );

  // Mock MCP server call function
  const callMCPServer = vi.fn();

  // Auth middleware
  const requireAuth = (req, res, next) => {
    if (!req.session.authenticated) {
      return res.status(401).json({ message: 'Authentication required' });
    }
    next();
  };

  // Routes
  app.post('/api/login', async (req, res) => {
    try {
      const { username, password } = req.body;

      if (!username || !password) {
        return res.status(400).json({ message: 'Username and password are required' });
      }

      const result = await callMCPServer('tools/call', {
        name: 'authenticate_user',
        arguments: { username, password },
      });

      if (!result.content || result.content.length === 0) {
        return res.status(401).json({ message: 'Invalid credentials' });
      }

      const content = result.content[0];
      const authData = JSON.parse(content.text);

      if (!authData || !authData.success) {
        return res.status(401).json({ message: authData.message || 'Invalid credentials' });
      }

      req.session.authenticated = true;
      req.session.user = username;
      req.session.mcpToken = authData.session_token;

      res.json({ user: username });
    } catch (error) {
      res.status(500).json({ message: 'Internal error' });
    }
  });

  app.post('/api/logout', (req, res) => {
    req.session.destroy((err) => {
      if (err) {
        return res.status(500).json({ message: 'Failed to logout' });
      }
      res.json({ message: 'Logged out successfully' });
    });
  });

  app.get('/api/auth/status', (req, res) => {
    if (req.session.authenticated) {
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

  app.get('/api/mcp/system-info', requireAuth, async (req, res) => {
    try {
      const result = await callMCPServer(
        'resources/read',
        { uri: 'pg://system_info' },
        req.session.mcpToken
      );

      if (!result.contents || result.contents.length === 0) {
        return res.status(404).json({ error: 'System info not found' });
      }

      const content = result.contents[0];
      let systemInfo;

      if (result.mimeType === 'application/json') {
        systemInfo = JSON.parse(content.text);
      } else {
        systemInfo = content.text;
      }

      res.json(systemInfo);
    } catch (error) {
      res.status(500).json({ error: error.message || 'Failed to fetch system info' });
    }
  });

  return { app, callMCPServer };
}

describe('Express Server API', () => {
  let app;
  let callMCPServer;

  beforeEach(() => {
    const testApp = createTestApp();
    app = testApp.app;
    callMCPServer = testApp.callMCPServer;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('POST /api/login', () => {
    it('returns 400 if username is missing', async () => {
      const response = await request(app)
        .post('/api/login')
        .send({ password: 'test' });

      expect(response.status).toBe(400);
      expect(response.body.message).toBe('Username and password are required');
    });

    it('returns 400 if password is missing', async () => {
      const response = await request(app)
        .post('/api/login')
        .send({ username: 'test' });

      expect(response.status).toBe(400);
      expect(response.body.message).toBe('Username and password are required');
    });

    it('returns 401 on invalid credentials', async () => {
      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: false,
              message: 'Invalid credentials',
            }),
          },
        ],
      });

      const response = await request(app)
        .post('/api/login')
        .send({ username: 'test', password: 'wrong' });

      expect(response.status).toBe(401);
      expect(response.body.message).toContain('Invalid credentials');
    });

    it('returns 200 and sets session on successful login', async () => {
      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: true,
              session_token: 'test-token-123',
            }),
          },
        ],
      });

      const response = await request(app)
        .post('/api/login')
        .send({ username: 'testuser', password: 'testpass' });

      expect(response.status).toBe(200);
      expect(response.body.user).toBe('testuser');
      expect(response.headers['set-cookie']).toBeDefined();
    });
  });

  describe('POST /api/logout', () => {
    it('destroys session and returns success', async () => {
      const agent = request.agent(app);

      // Login first
      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: true,
              session_token: 'test-token',
            }),
          },
        ],
      });

      await agent
        .post('/api/login')
        .send({ username: 'test', password: 'test' });

      // Then logout
      const response = await agent.post('/api/logout');

      expect(response.status).toBe(200);
      expect(response.body.message).toBe('Logged out successfully');
    });
  });

  describe('GET /api/auth/status', () => {
    it('returns authenticated false when not logged in', async () => {
      const response = await request(app).get('/api/auth/status');

      expect(response.status).toBe(200);
      expect(response.body.authenticated).toBe(false);
    });

    it('returns authenticated true when logged in', async () => {
      const agent = request.agent(app);

      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: true,
              session_token: 'test-token',
            }),
          },
        ],
      });

      await agent
        .post('/api/login')
        .send({ username: 'testuser', password: 'testpass' });

      const response = await agent.get('/api/auth/status');

      expect(response.status).toBe(200);
      expect(response.body.authenticated).toBe(true);
      expect(response.body.user).toBe('testuser');
    });
  });

  describe('GET /api/mcp/system-info', () => {
    it('returns 401 when not authenticated', async () => {
      const response = await request(app).get('/api/mcp/system-info');

      expect(response.status).toBe(401);
      expect(response.body.message).toBe('Authentication required');
    });

    it('returns system info when authenticated', async () => {
      const agent = request.agent(app);

      // Login first
      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: true,
              session_token: 'test-token',
            }),
          },
        ],
      });

      await agent
        .post('/api/login')
        .send({ username: 'test', password: 'test' });

      // Mock system info response
      const mockSystemInfo = {
        postgresql_version: '17.4',
        operating_system: 'linux',
        architecture: 'x86_64',
        bit_version: '64-bit',
      };

      callMCPServer.mockResolvedValueOnce({
        mimeType: 'application/json',
        contents: [
          {
            type: 'text',
            text: JSON.stringify(mockSystemInfo),
          },
        ],
      });

      const response = await agent.get('/api/mcp/system-info');

      expect(response.status).toBe(200);
      expect(response.body).toEqual(mockSystemInfo);
    });

    it('returns 404 when system info not found', async () => {
      const agent = request.agent(app);

      // Login first
      callMCPServer.mockResolvedValueOnce({
        content: [
          {
            type: 'text',
            text: JSON.stringify({
              success: true,
              session_token: 'test-token',
            }),
          },
        ],
      });

      await agent
        .post('/api/login')
        .send({ username: 'test', password: 'test' });

      // Mock empty response
      callMCPServer.mockResolvedValueOnce({
        contents: [],
      });

      const response = await agent.get('/api/mcp/system-info');

      expect(response.status).toBe(404);
      expect(response.body.error).toBe('System info not found');
    });
  });
});
