/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Markdown Components
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
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
export const createMarkdownComponents = (theme) => {
    const isDark = theme.palette.mode === 'dark';

    return {
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
                        borderRadius: '8px',
                        fontSize: '0.875rem',
                        border: isDark ? '1px solid #334155' : '1px solid #E5E7EB',
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
                        backgroundColor: isDark ? 'rgba(51, 65, 85, 0.5)' : 'rgba(229, 231, 235, 0.5)',
                        padding: '2px 6px',
                        borderRadius: '4px',
                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                        fontSize: '0.875em',
                        color: isDark ? '#22B8CF' : '#15AABF',
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
                        fontSize: '1.125rem',
                        lineHeight: 1.6,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                    }}
                >
                    {children}
                </p>
            );
        },

        h1({ children }) {
            return (
                <Typography
                    variant="h5"
                    sx={{
                        mt: 2,
                        mb: 1,
                        fontWeight: 600,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                    }}
                >
                    {children}
                </Typography>
            );
        },

        h2({ children }) {
            return (
                <Typography
                    variant="h6"
                    sx={{
                        mt: 2,
                        mb: 1,
                        fontWeight: 600,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                    }}
                >
                    {children}
                </Typography>
            );
        },

        h3({ children }) {
            return (
                <Typography
                    variant="subtitle1"
                    sx={{
                        mt: 1.5,
                        mb: 1,
                        fontWeight: 600,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                    }}
                >
                    {children}
                </Typography>
            );
        },

        ul({ children }) {
            return (
                <ul style={{
                    paddingLeft: theme.spacing(2),
                    marginTop: theme.spacing(1),
                    marginBottom: theme.spacing(1),
                    color: isDark ? '#F1F5F9' : '#1F2937',
                }}>
                    {children}
                </ul>
            );
        },

        ol({ children }) {
            return (
                <ol style={{
                    paddingLeft: theme.spacing(2),
                    marginTop: theme.spacing(1),
                    marginBottom: theme.spacing(1),
                    color: isDark ? '#F1F5F9' : '#1F2937',
                }}>
                    {children}
                </ol>
            );
        },

        li({ children }) {
            return (
                <li
                    style={{
                        marginBottom: theme.spacing(0.5),
                        fontSize: '1.125rem',
                        lineHeight: 1.6,
                        color: isDark ? '#F1F5F9' : '#1F2937',
                    }}
                >
                    {children}
                </li>
            );
        },

        a({ href, children }) {
            return (
                <a
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{
                        color: isDark ? '#22B8CF' : '#15AABF',
                        textDecoration: 'none',
                        borderBottom: `1px solid ${isDark ? '#22B8CF' : '#15AABF'}`,
                    }}
                >
                    {children}
                </a>
            );
        },

        table({ children }) {
            return (
                <Box sx={{ overflowX: 'auto', my: 2 }}>
                    <table style={{
                        borderCollapse: 'collapse',
                        width: '100%',
                        border: `1px solid ${isDark ? '#334155' : '#E5E7EB'}`,
                        borderRadius: '8px',
                    }}>
                        {children}
                    </table>
                </Box>
            );
        },

        th({ children }) {
            return (
                <th style={{
                    border: `1px solid ${isDark ? '#334155' : '#E5E7EB'}`,
                    padding: '10px 12px',
                    backgroundColor: isDark ? '#1E293B' : '#F9FAFB',
                    color: isDark ? '#F1F5F9' : '#1F2937',
                    fontWeight: 600,
                    textAlign: 'left',
                    fontSize: '1rem',
                }}>
                    {children}
                </th>
            );
        },

        td({ children }) {
            return (
                <td style={{
                    border: `1px solid ${isDark ? '#334155' : '#E5E7EB'}`,
                    padding: '10px 12px',
                    color: isDark ? '#F1F5F9' : '#1F2937',
                    fontSize: '1rem',
                }}>
                    {children}
                </td>
            );
        },

        blockquote({ children }) {
            return (
                <blockquote
                    style={{
                        borderLeft: `4px solid ${isDark ? '#22B8CF' : '#15AABF'}`,
                        margin: '1em 0',
                        paddingLeft: '1em',
                        color: isDark ? '#94A3B8' : '#6B7280',
                        fontStyle: 'italic',
                    }}
                >
                    {children}
                </blockquote>
            );
        },

        hr() {
            return (
                <hr
                    style={{
                        border: 'none',
                        borderTop: `1px solid ${isDark ? '#334155' : '#E5E7EB'}`,
                        margin: '1.5em 0',
                    }}
                />
            );
        },
    };
};
