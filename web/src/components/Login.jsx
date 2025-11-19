/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState } from 'react';
import {
  Box,
  Card,
  CardContent,
  TextField,
  Button,
  Typography,
  Alert,
  Container,
  keyframes,
} from '@mui/material';
import { useAuth } from '../contexts/AuthContext';
import logoLight from '../assets/images/logo-light.png';

// Keyframe animations for abstract shapes
const float = keyframes`
  0%, 100% { transform: translateY(0px) rotate(0deg); }
  50% { transform: translateY(-30px) rotate(10deg); }
`;

const float2 = keyframes`
  0%, 100% { transform: translateY(0px) translateX(0px) rotate(0deg); }
  33% { transform: translateY(-40px) translateX(20px) rotate(120deg); }
  66% { transform: translateY(20px) translateX(-20px) rotate(240deg); }
`;

const float3 = keyframes`
  0%, 100% { transform: translateY(0px) rotate(0deg) scale(1); }
  50% { transform: translateY(-20px) rotate(-15deg) scale(1.1); }
`;

const gradientShift = keyframes`
  0% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
`;

const rotate = keyframes`
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
`;

const Login = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [warning, setWarning] = useState('');
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();

  // Check for disconnect message on mount
  React.useEffect(() => {
    const disconnectMsg = sessionStorage.getItem('disconnectMessage');
    if (disconnectMsg) {
      setWarning(disconnectMsg);
      sessionStorage.removeItem('disconnectMessage');
    }
  }, []);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setWarning('');
    setLoading(true);

    try {
      await login(username, password);
    } catch (err) {
      setError(err.message || 'Failed to login');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        overflow: 'hidden',
        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 25%, #f093fb 50%, #4facfe 75%, #00f2fe 100%)',
        backgroundSize: '400% 400%',
        animation: `${gradientShift} 15s ease infinite`,
      }}
    >
      {/* Abstract floating shapes */}
      <Box
        sx={{
          position: 'absolute',
          top: '10%',
          left: '10%',
          width: '300px',
          height: '300px',
          borderRadius: '50%',
          background: 'linear-gradient(45deg, rgba(255,107,107,0.3), rgba(255,175,123,0.3))',
          filter: 'blur(60px)',
          animation: `${float} 8s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          top: '60%',
          right: '15%',
          width: '250px',
          height: '250px',
          borderRadius: '30% 70% 70% 30% / 30% 30% 70% 70%',
          background: 'linear-gradient(225deg, rgba(108,92,231,0.4), rgba(162,155,254,0.4))',
          filter: 'blur(50px)',
          animation: `${float2} 12s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          bottom: '20%',
          left: '20%',
          width: '200px',
          height: '200px',
          borderRadius: '63% 37% 54% 46% / 55% 48% 52% 45%',
          background: 'linear-gradient(135deg, rgba(79,172,254,0.4), rgba(0,242,254,0.4))',
          filter: 'blur(40px)',
          animation: `${float3} 10s ease-in-out infinite`,
        }}
      />

      {/* Rotating geometric shapes */}
      <Box
        sx={{
          position: 'absolute',
          top: '30%',
          right: '25%',
          width: '150px',
          height: '150px',
          border: '3px solid rgba(255,255,255,0.2)',
          borderRadius: '20px',
          transform: 'rotate(45deg)',
          animation: `${rotate} 20s linear infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          bottom: '35%',
          right: '10%',
          width: '100px',
          height: '100px',
          border: '2px solid rgba(255,255,255,0.15)',
          borderRadius: '50%',
          animation: `${float} 6s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          top: '15%',
          right: '35%',
          width: '80px',
          height: '80px',
          background: 'rgba(255,255,255,0.1)',
          borderRadius: '10px',
          transform: 'rotate(30deg)',
          animation: `${float3} 9s ease-in-out infinite`,
        }}
      />

      {/* Additional geometric shapes */}
      <Box
        sx={{
          position: 'absolute',
          top: '5%',
          left: '30%',
          width: '120px',
          height: '120px',
          border: '2px solid rgba(255,255,255,0.18)',
          borderRadius: '50%',
          animation: `${float2} 14s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          bottom: '15%',
          right: '30%',
          width: '90px',
          height: '90px',
          border: '3px solid rgba(255,255,255,0.15)',
          borderRadius: '15px',
          transform: 'rotate(20deg)',
          animation: `${rotate} 25s linear infinite reverse`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          top: '45%',
          left: '8%',
          width: '70px',
          height: '70px',
          background: 'rgba(255,255,255,0.08)',
          borderRadius: '50%',
          animation: `${float} 7s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          bottom: '25%',
          left: '35%',
          width: '60px',
          height: '60px',
          border: '2px solid rgba(255,255,255,0.2)',
          borderRadius: '8px',
          transform: 'rotate(60deg)',
          animation: `${float3} 11s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          top: '55%',
          right: '5%',
          width: '110px',
          height: '110px',
          border: '3px solid rgba(255,255,255,0.12)',
          borderRadius: '50%',
          animation: `${float2} 16s ease-in-out infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          top: '70%',
          left: '15%',
          width: '50px',
          height: '50px',
          background: 'rgba(255,255,255,0.12)',
          borderRadius: '5px',
          transform: 'rotate(15deg)',
          animation: `${rotate} 18s linear infinite`,
        }}
      />

      <Box
        sx={{
          position: 'absolute',
          bottom: '45%',
          right: '18%',
          width: '95px',
          height: '95px',
          border: '2px solid rgba(255,255,255,0.16)',
          borderRadius: '18px',
          transform: 'rotate(35deg)',
          animation: `${float} 13s ease-in-out infinite`,
        }}
      />

      <Container maxWidth="sm" sx={{ position: 'relative', zIndex: 1 }}>
        <Card
          elevation={24}
          sx={{
            backdropFilter: 'blur(20px)',
            backgroundColor: 'rgba(255, 255, 255, 0.95)',
            borderRadius: 4,
            overflow: 'visible',
          }}
        >
          <CardContent sx={{ p: 4 }}>
            <Box sx={{ textAlign: 'center', mb: 4 }}>
              <Box
                component="img"
                src={logoLight}
                alt="pgEdge"
                sx={{
                  height: '60px',
                  mb: 2,
                  filter: 'drop-shadow(0 4px 6px rgba(0,0,0,0.1))',
                }}
              />
              <Typography
                variant="h4"
                component="h1"
                gutterBottom
                sx={{ fontWeight: 600 }}
              >
                MCP Client
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Sign in to continue
              </Typography>
            </Box>

            {warning && (
              <Alert severity="warning" sx={{ mb: 2 }} onClose={() => setWarning('')}>
                {warning}
              </Alert>
            )}

            {error && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {error}
              </Alert>
            )}

            <form onSubmit={handleSubmit} noValidate>
              <TextField
                fullWidth
                label="Username"
                type="text"
                name="username"
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                margin="normal"
                required
                autoFocus
                disabled={loading}
                inputProps={{
                  autoComplete: 'off',
                }}
              />

              <TextField
                fullWidth
                label="Password"
                type="password"
                name="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                margin="normal"
                required
                disabled={loading}
                inputProps={{
                  autoComplete: 'current-password',
                }}
              />

              <Button
                fullWidth
                type="submit"
                variant="contained"
                size="large"
                sx={{ mt: 3 }}
                disabled={loading}
              >
                {loading ? 'Signing in...' : 'Sign In'}
              </Button>
            </form>

            <Box sx={{ mt: 3, textAlign: 'center' }}>
              <Typography variant="caption" color="text.secondary">
                Contact your administrator to create an account
              </Typography>
            </Box>
          </CardContent>
        </Card>
      </Container>
    </Box>
  );
};

export default Login;
