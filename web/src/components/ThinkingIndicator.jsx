/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Thinking Indicator Component
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import React, { useState, useEffect, useRef } from 'react';
import PropTypes from 'prop-types';
import { Box, CircularProgress, Typography } from '@mui/material';

// PostgreSQL/Elephant themed action words for thinking animation
const ELEPHANT_ACTIONS = [
    "Thinking with trunks",
    "Consulting the herd",
    "Stampeding through data",
    "Trumpeting queries",
    "Migrating thoughts",
    "Packing memories",
    "Charging through logic",
    "Bathing in wisdom",
    "Roaming the database",
    "Grazing on metadata",
    "Herding ideas",
    "Splashing in pools",
    "Foraging for answers",
    "Wandering savannah",
    "Dusting off schemas",
    "Pondering profoundly",
    "Remembering everything",
    "Trumpeting brilliance",
    "Stomping bugs",
    "Tusking through code",
];

const ThinkingIndicator = ({ isThinking }) => {
    const [message, setMessage] = useState('');
    const intervalRef = useRef(null);

    useEffect(() => {
        if (isThinking) {
            // Set initial random message
            setMessage(ELEPHANT_ACTIONS[Math.floor(Math.random() * ELEPHANT_ACTIONS.length)]);

            // Change message every 2 seconds
            intervalRef.current = setInterval(() => {
                setMessage(ELEPHANT_ACTIONS[Math.floor(Math.random() * ELEPHANT_ACTIONS.length)]);
            }, 2000);
        } else {
            // Clear interval and message when not thinking
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
            setMessage('');
        }

        // Cleanup on unmount
        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
            }
        };
    }, [isThinking]);

    if (!isThinking) return null;

    return (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <CircularProgress size={20} />
            <Typography
                variant="body2"
                sx={{
                    color: 'text.secondary',
                    fontStyle: 'italic',
                }}
            >
                {message}...
            </Typography>
        </Box>
    );
};

ThinkingIndicator.propTypes = {
    isThinking: PropTypes.bool.isRequired,
};

export default ThinkingIndicator;
