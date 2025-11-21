/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Markdown Components
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import { Box, Typography, useTheme } from '@mui/material';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

/**
 * Custom components for rendering markdown with Material-UI styling
 * @param {Object} theme - MUI theme object
 * @returns {Object} Component overrides for react-markdown
 */
export const createMarkdownComponents = (theme) => ({
    code({ node, inline, className, children, ...props }) {
        const match = /language-(\w+)/.exec(className || '');
        const language = match ? match[1] : '';

        // Check if this is truly inline code by checking for newlines
        const childText = String(children);
        const isInline = inline || !childText.includes('\n');

        return !isInline ? (
            <SyntaxHighlighter
                style={vscDarkPlus}
                language={language || 'text'}
                PreTag="div"
                customStyle={{
                    margin: '1em 0',
                    borderRadius: '4px',
                    fontSize: '0.875rem',
                }}
                {...props}
            >
                {String(children).replace(/\n$/, '')}
            </SyntaxHighlighter>
        ) : (
            <code
                {...props}
                style={{
                    display: 'inline',
                    verticalAlign: 'baseline',
                    backgroundColor: theme.palette.mode === 'dark' ? 'rgba(255, 255, 255, 0.1)' : 'rgba(0, 0, 0, 0.1)',
                    padding: '2px 6px',
                    borderRadius: '3px',
                    fontFamily: 'monospace',
                    fontSize: '0.875em',
                }}
            >
                {children}
            </code>
        );
    },

    pre({ children }) {
        return <>{children}</>;
    },

    p({ children }) {
        return (
            <p
                style={{
                    marginBottom: theme.spacing(1),
                    fontSize: '1rem',
                    lineHeight: 1.5,
                    color: theme.palette.text.primary,
                }}
            >
                {children}
            </p>
        );
    },

    h1({ children }) {
        return <Typography variant="h5" sx={{ mt: 2, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
    },

    h2({ children }) {
        return <Typography variant="h6" sx={{ mt: 2, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
    },

    h3({ children }) {
        return <Typography variant="subtitle1" sx={{ mt: 1.5, mb: 1, fontWeight: 'bold' }}>{children}</Typography>;
    },

    ul({ children }) {
        return (
            <ul style={{ paddingLeft: theme.spacing(2), marginTop: theme.spacing(1), marginBottom: theme.spacing(1) }}>
                {children}
            </ul>
        );
    },

    ol({ children }) {
        return (
            <ol style={{ paddingLeft: theme.spacing(2), marginTop: theme.spacing(1), marginBottom: theme.spacing(1) }}>
                {children}
            </ol>
        );
    },

    li({ children }) {
        return (
            <li
                style={{
                    marginBottom: theme.spacing(0.5),
                    fontSize: '1rem',
                    lineHeight: 1.5,
                    color: theme.palette.text.primary,
                }}
            >
                {children}
            </li>
        );
    },

    a({ href, children }) {
        return (
            <a href={href} target="_blank" rel="noopener noreferrer" style={{ color: '#1976d2' }}>
                {children}
            </a>
        );
    },

    table({ children }) {
        return (
            <Box sx={{ overflowX: 'auto', my: 2 }}>
                <table style={{ borderCollapse: 'collapse', width: '100%' }}>{children}</table>
            </Box>
        );
    },

    th({ children }) {
        return (
            <th style={{
                border: `1px solid ${theme.palette.mode === 'dark' ? '#555' : '#ddd'}`,
                padding: '8px',
                backgroundColor: theme.palette.mode === 'dark' ? '#2a2a2a' : '#f5f5f5',
                color: theme.palette.mode === 'dark' ? '#fff' : '#000',
                fontWeight: 'bold',
                textAlign: 'left',
            }}>
                {children}
            </th>
        );
    },

    td({ children }) {
        return (
            <td style={{
                border: `1px solid ${theme.palette.mode === 'dark' ? '#555' : '#ddd'}`,
                padding: '8px',
            }}>
                {children}
            </td>
        );
    },
});
