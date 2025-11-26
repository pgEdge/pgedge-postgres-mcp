/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import { Box } from '@mui/material';
import { LLMProcessingProvider } from '../contexts/LLMProcessingContext';
import StatusBanner from './StatusBanner';
import ChatInterface from './ChatInterface';

const MainContent = () => {
    return (
        <LLMProcessingProvider>
            <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
                <StatusBanner />
                <ChatInterface />
            </Box>
        </LLMProcessingProvider>
    );
};

export default MainContent;
