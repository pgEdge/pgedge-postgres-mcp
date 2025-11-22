/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Help Panel
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import {
    Drawer,
    Box,
    Typography,
    IconButton,
    Divider,
    List,
    ListItem,
    ListItemText,
} from '@mui/material';
import { Close as CloseIcon } from '@mui/icons-material';

const HelpPanel = ({ open, onClose }) => {
    return (
        <Drawer
            anchor="right"
            open={open}
            onClose={onClose}
            sx={{
                '& .MuiDrawer-paper': {
                    width: { xs: '100%', sm: 500 },
                    p: 3,
                },
            }}
        >
            <Box>
                {/* Header */}
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
                    <Typography variant="h5" component="h2">
                        Help & Documentation
                    </Typography>
                    <IconButton onClick={onClose} aria-label="close help">
                        <CloseIcon />
                    </IconButton>
                </Box>

                <Divider sx={{ mb: 3 }} />

                {/* Getting Started */}
                <Typography variant="h6" gutterBottom>
                    Getting Started
                </Typography>
                <Typography variant="body2" paragraph>
                    The pgEdge Natural Language Agent allows you to interact with your PostgreSQL database using natural language.
                    Ask questions about your data, run queries, and get insights without writing SQL.
                </Typography>

                <Divider sx={{ my: 3 }} />

                {/* Chat Interface */}
                <Typography variant="h6" gutterBottom>
                    Chat Interface
                </Typography>
                <List dense>
                    <ListItem>
                        <ListItemText
                            primary="Sending Messages"
                            secondary="Type your question in the input box and press Enter or click the send button. Use Shift+Enter for new lines."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Query History"
                            secondary="Use the up and down arrow keys to navigate through your previous queries."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Clear Conversation"
                            secondary="Click the 'Clear' button at the top of the chat to start a new conversation."
                        />
                    </ListItem>
                </List>

                <Divider sx={{ my: 3 }} />

                {/* Prompts */}
                <Typography variant="h6" gutterBottom>
                    Prompts
                </Typography>
                <Typography variant="body2" paragraph>
                    Prompts are guided workflows that help you perform complex multi-step tasks.
                    Click the brain icon (next to the send button) to access available prompts.
                </Typography>
                <List dense>
                    <ListItem>
                        <ListItemText
                            primary="setup-semantic-search"
                            secondary="Guides you through setting up semantic search on your database. Discovers vector-capable tables and executes optimized searches."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="explore-database"
                            secondary="Systematically explores your database structure, identifying tables, relationships, and capabilities like semantic search."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="diagnose-query-issue"
                            secondary="Helps diagnose why queries aren't working. Checks connection, schema, data availability, and common issues."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Using Prompts"
                            secondary="Select a prompt from the dropdown, fill in any required arguments, and click 'Execute Prompt'. The AI will follow the guided workflow."
                        />
                    </ListItem>
                </List>

                <Divider sx={{ my: 3 }} />

                {/* Settings */}
                <Typography variant="h6" gutterBottom>
                    Settings & Options
                </Typography>
                <List dense>
                    <ListItem>
                        <ListItemText
                            primary="LLM Provider"
                            secondary="Select your preferred AI provider (Anthropic, OpenAI, or Ollama) from the dropdown."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Model Selection"
                            secondary="Choose the specific AI model to use. Different models have different capabilities and speeds."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Show Activity"
                            secondary="Toggle to show/hide the tools and resources being used by the AI to answer your questions."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Render Markdown"
                            secondary="Toggle to enable/disable markdown rendering for AI responses. When off, responses are shown as plain text."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Theme"
                            secondary="Click the sun/moon icon in the header to switch between light and dark mode."
                        />
                    </ListItem>
                </List>

                <Divider sx={{ my: 3 }} />

                {/* Tips */}
                <Typography variant="h6" gutterBottom>
                    Tips & Best Practices
                </Typography>
                <List dense>
                    <ListItem>
                        <ListItemText
                            primary="Be Specific"
                            secondary="The more specific your question, the better the response. Include table names, column names, or conditions when relevant."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Follow-up Questions"
                            secondary="The AI maintains conversation context, so you can ask follow-up questions that reference previous responses."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Review Activity"
                            secondary="Keep 'Show Activity' enabled to see which database operations the AI is performing on your behalf."
                        />
                    </ListItem>
                    <ListItem>
                        <ListItemText
                            primary="Preferences Saved"
                            secondary="Your theme, provider, model, and toggle settings are automatically saved and restored on your next visit."
                        />
                    </ListItem>
                </List>

                <Divider sx={{ my: 3 }} />

                {/* Database Info */}
                <Typography variant="h6" gutterBottom>
                    Database Connection
                </Typography>
                <Typography variant="body2" paragraph>
                    Connection details are shown in the status banner at the top of the page.
                    A green indicator means you're connected. Click the banner to expand/collapse detailed connection information.
                </Typography>

                <Divider sx={{ my: 3 }} />

                {/* Version Info */}
                <Typography variant="body2" color="text.secondary" sx={{ mt: 4 }}>
                    pgEdge Natural Language Agent
                    <br />
                    Copyright &copy; 2025, pgEdge, Inc.
                </Typography>
            </Box>
        </Drawer>
    );
};

export default HelpPanel;
