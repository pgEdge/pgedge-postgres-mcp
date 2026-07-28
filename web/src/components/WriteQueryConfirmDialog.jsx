/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Write Query Confirmation Dialog
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

import PropTypes from 'prop-types';
import {
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    Button,
    Box,
    Typography,
    useTheme,
    alpha,
} from '@mui/material';
import {
    Warning as WarningIcon,
    Close as CloseIcon,
    PlayArrow as PlayArrowIcon,
} from '@mui/icons-material';

const WriteQueryConfirmDialog = ({
    open,
    onClose,
    onConfirm,
    query = '',
}) => {
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    return (
        <Dialog
            open={open}
            onClose={onClose}
            maxWidth="sm"
            fullWidth
            aria-labelledby="write-query-confirm-dialog-title"
            PaperProps={{
                sx: {
                    bgcolor: isDark ? '#1E293B' : '#FFFFFF',
                    border: '1px solid',
                    borderColor: isDark ? '#334155' : '#E5E7EB',
                    borderRadius: 1,
                },
            }}
        >
            <DialogTitle id="write-query-confirm-dialog-title">
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <WarningIcon sx={{ color: '#F59E0B' }} />
                    <Typography
                        variant="h6"
                        component="span"
                        sx={{ color: isDark ? '#F1F5F9' : '#1F2937' }}
                    >
                        Confirm Write Operation
                    </Typography>
                </Box>
            </DialogTitle>

            <DialogContent>
                <Typography
                    variant="body2"
                    sx={{
                        color: isDark ? '#94A3B8' : '#6B7280',
                        mb: 2,
                    }}
                >
                    The following operation can modify data:
                </Typography>
                <Box
                    sx={{
                        p: 2,
                        bgcolor: isDark ? '#0F172A' : '#F9FAFB',
                        borderRadius: 1,
                        border: '1px solid',
                        borderColor: isDark ? '#334155' : '#E5E7EB',
                        fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                        fontSize: '0.8rem',
                        color: isDark ? '#F1F5F9' : '#1F2937',
                        whiteSpace: 'pre-wrap',
                        wordBreak: 'break-word',
                        maxHeight: 300,
                        overflow: 'auto',
                    }}
                >
                    {query}
                </Box>
            </DialogContent>

            <DialogActions sx={{ p: 2, pt: 0 }}>
                <Button
                    onClick={onClose}
                    startIcon={<CloseIcon />}
                    sx={{
                        color: isDark ? '#94A3B8' : '#6B7280',
                        textTransform: 'none',
                    }}
                >
                    Cancel
                </Button>
                <Button
                    onClick={onConfirm}
                    variant="contained"
                    startIcon={<PlayArrowIcon />}
                    sx={{
                        bgcolor: isDark
                            ? alpha('#F59E0B', 0.9)
                            : '#F59E0B',
                        color: isDark ? '#0F172A' : '#FFFFFF',
                        textTransform: 'none',
                        fontWeight: 600,
                        '&:hover': {
                            bgcolor: isDark
                                ? '#F59E0B'
                                : '#D97706',
                        },
                    }}
                >
                    Execute
                </Button>
            </DialogActions>
        </Dialog>
    );
};

WriteQueryConfirmDialog.propTypes = {
    open: PropTypes.bool.isRequired,
    onClose: PropTypes.func.isRequired,
    onConfirm: PropTypes.func.isRequired,
    query: PropTypes.string,
};

export default WriteQueryConfirmDialog;
