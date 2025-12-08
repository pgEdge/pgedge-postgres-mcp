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
import { DatabaseProvider } from '../contexts/DatabaseContext';
import StatusBanner from './StatusBanner';
import ChatInterface from './ChatInterface';

const MainContent = ({ conversations }) => {
    return (
        <DatabaseProvider>
            <LLMProcessingProvider>
                <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
                    <StatusBanner />
                    <ChatInterface conversations={conversations} />
                </Box>
            </LLMProcessingProvider>
        </DatabaseProvider>
    );
};

export default MainContent;
