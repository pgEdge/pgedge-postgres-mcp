/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React from 'react';
import { Box } from '@mui/material';
import StatusBanner from './StatusBanner';
import ChatInterface from './ChatInterface';

const MainContent = () => {
    return (
        <Box>
            <StatusBanner />
            <ChatInterface />
        </Box>
    );
};

export default MainContent;
