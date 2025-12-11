/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Conversation Panel
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Styled to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import React, { useState } from 'react';
import {
    Drawer,
    Box,
    Typography,
    IconButton,
    List,
    ListItem,
    ListItemButton,
    ListItemText,
    Divider,
    Button,
    CircularProgress,
    Tooltip,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogContentText,
    DialogActions,
    TextField,
    useTheme,
    alpha,
} from '@mui/material';
import {
    Close as CloseIcon,
    Add as AddIcon,
    Delete as DeleteIcon,
    Edit as EditIcon,
    Chat as ChatIcon,
    DeleteSweep as DeleteSweepIcon,
} from '@mui/icons-material';

/**
 * Format a date relative to now
 * @param {string|Date} date - Date to format
 * @returns {string} - Formatted date string
 */
const formatRelativeDate = (date) => {
    const d = new Date(date);
    const now = new Date();
    const diffMs = now - d;
    const diffMins = Math.floor(diffMs / (1000 * 60));
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return d.toLocaleDateString();
};

const ConversationPanel = ({
    open,
    onClose,
    conversations,
    currentConversationId,
    onSelect,
    onNewConversation,
    onRename,
    onDelete,
    onDeleteAll,
    loading,
    disabled,
}) => {
    const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
    const [conversationToDelete, setConversationToDelete] = useState(null);
    const [deleteAllDialogOpen, setDeleteAllDialogOpen] = useState(false);
    const [renameDialogOpen, setRenameDialogOpen] = useState(false);
    const [conversationToRename, setConversationToRename] = useState(null);
    const [newTitle, setNewTitle] = useState('');
    const theme = useTheme();
    const isDark = theme.palette.mode === 'dark';

    const handleDeleteClick = (e, conversation) => {
        e.stopPropagation();
        setConversationToDelete(conversation);
        setDeleteDialogOpen(true);
    };

    const handleConfirmDelete = () => {
        if (conversationToDelete) {
            onDelete(conversationToDelete.id);
        }
        setDeleteDialogOpen(false);
        setConversationToDelete(null);
    };

    const handleCancelDelete = () => {
        setDeleteDialogOpen(false);
        setConversationToDelete(null);
    };

    const handleDeleteAllClick = () => {
        setDeleteAllDialogOpen(true);
    };

    const handleConfirmDeleteAll = () => {
        onDeleteAll();
        setDeleteAllDialogOpen(false);
    };

    const handleCancelDeleteAll = () => {
        setDeleteAllDialogOpen(false);
    };

    const handleRenameClick = (e, conversation) => {
        e.stopPropagation();
        setConversationToRename(conversation);
        setNewTitle(conversation.title);
        setRenameDialogOpen(true);
    };

    const handleConfirmRename = () => {
        if (conversationToRename && newTitle.trim()) {
            onRename(conversationToRename.id, newTitle.trim());
        }
        setRenameDialogOpen(false);
        setConversationToRename(null);
        setNewTitle('');
    };

    const handleCancelRename = () => {
        setRenameDialogOpen(false);
        setConversationToRename(null);
        setNewTitle('');
    };

    const dialogPaperProps = {
        sx: {
            bgcolor: isDark ? '#1E293B' : '#FFFFFF',
            border: '1px solid',
            borderColor: isDark ? '#334155' : '#E5E7EB',
            borderRadius: 1,
        },
    };

    return (
        <>
            <Drawer
                anchor="left"
                open={open}
                onClose={onClose}
                sx={{
                    '& .MuiDrawer-paper': {
                        width: { xs: '100%', sm: 320 },
                        boxSizing: 'border-box',
                        bgcolor: isDark ? '#0F172A' : '#FFFFFF',
                        borderRight: '1px solid',
                        borderColor: isDark ? '#334155' : '#E5E7EB',
                    },
                }}
            >
                <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
                    {/* Header */}
                    <Box
                        sx={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            p: 2,
                            borderBottom: '1px solid',
                            borderColor: isDark ? '#334155' : '#E5E7EB',
                        }}
                    >
                        <Typography
                            variant="h6"
                            component="h2"
                            sx={{
                                color: isDark ? '#F1F5F9' : '#1F2937',
                                fontWeight: 600,
                            }}
                        >
                            Conversations
                        </Typography>
                        <IconButton
                            onClick={onClose}
                            aria-label="close panel"
                            sx={{
                                color: isDark ? '#94A3B8' : '#6B7280',
                                '&:hover': {
                                    bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                    color: '#15AABF',
                                },
                            }}
                        >
                            <CloseIcon />
                        </IconButton>
                    </Box>

                    {/* New Conversation Button */}
                    <Box sx={{ p: 2 }}>
                        <Button
                            variant="contained"
                            startIcon={<AddIcon />}
                            onClick={() => {
                                onNewConversation();
                                onClose();
                            }}
                            fullWidth
                            disabled={disabled}
                            sx={{
                                bgcolor: '#15AABF',
                                color: '#FFFFFF',
                                borderRadius: 1,
                                textTransform: 'none',
                                fontWeight: 500,
                                fontSize: '1rem',
                                py: 1,
                                '&:hover': {
                                    bgcolor: '#0C8599',
                                },
                                '&.Mui-disabled': {
                                    bgcolor: isDark ? '#334155' : '#E5E7EB',
                                    color: isDark ? '#64748B' : '#9CA3AF',
                                },
                            }}
                        >
                            New Conversation
                        </Button>
                    </Box>

                    <Divider sx={{ borderColor: isDark ? '#334155' : '#E5E7EB' }} />

                    {/* Conversation List */}
                    <Box sx={{ flex: 1, overflow: 'auto' }}>
                        {loading ? (
                            <Box
                                sx={{
                                    display: 'flex',
                                    justifyContent: 'center',
                                    alignItems: 'center',
                                    height: '100%',
                                    minHeight: 200,
                                }}
                            >
                                <CircularProgress size={32} sx={{ color: '#15AABF' }} />
                            </Box>
                        ) : conversations.length === 0 ? (
                            <Box
                                sx={{
                                    display: 'flex',
                                    flexDirection: 'column',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                    height: '100%',
                                    minHeight: 200,
                                    p: 3,
                                    textAlign: 'center',
                                }}
                            >
                                <Box
                                    sx={{
                                        width: 64,
                                        height: 64,
                                        borderRadius: '50%',
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                        bgcolor: isDark ? alpha('#22B8CF', 0.1) : alpha('#15AABF', 0.08),
                                        mb: 2,
                                    }}
                                >
                                    <ChatIcon sx={{ fontSize: 32, color: isDark ? '#22B8CF' : '#15AABF' }} />
                                </Box>
                                <Typography
                                    variant="body1"
                                    sx={{ color: isDark ? '#F1F5F9' : '#1F2937', fontWeight: 500 }}
                                >
                                    No conversations yet
                                </Typography>
                                <Typography
                                    variant="body2"
                                    sx={{ color: isDark ? '#64748B' : '#9CA3AF', mt: 1 }}
                                >
                                    Start a new conversation to begin
                                </Typography>
                            </Box>
                        ) : (
                            <List disablePadding>
                                {conversations.map((conversation) => (
                                    <ListItem
                                        key={conversation.id}
                                        disablePadding
                                        secondaryAction={
                                            <Box sx={{ display: 'flex', gap: 0.5 }}>
                                                <Tooltip title="Rename conversation">
                                                    <IconButton
                                                        aria-label="rename"
                                                        onClick={(e) => handleRenameClick(e, conversation)}
                                                        disabled={disabled}
                                                        size="small"
                                                        sx={{
                                                            color: isDark ? '#94A3B8' : '#6B7280',
                                                            '&:hover': {
                                                                bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                                                                color: '#15AABF',
                                                            },
                                                        }}
                                                    >
                                                        <EditIcon fontSize="small" />
                                                    </IconButton>
                                                </Tooltip>
                                                <Tooltip title="Delete conversation">
                                                    <IconButton
                                                        edge="end"
                                                        aria-label="delete"
                                                        onClick={(e) => handleDeleteClick(e, conversation)}
                                                        disabled={disabled}
                                                        size="small"
                                                        sx={{
                                                            color: isDark ? '#94A3B8' : '#6B7280',
                                                            '&:hover': {
                                                                bgcolor: isDark ? alpha('#EF4444', 0.08) : alpha('#EF4444', 0.04),
                                                                color: '#EF4444',
                                                            },
                                                        }}
                                                    >
                                                        <DeleteIcon fontSize="small" />
                                                    </IconButton>
                                                </Tooltip>
                                            </Box>
                                        }
                                        sx={{
                                            '& .MuiListItemSecondaryAction-root': {
                                                opacity: 0,
                                                transition: 'opacity 0.2s',
                                            },
                                            '&:hover .MuiListItemSecondaryAction-root': {
                                                opacity: 1,
                                            },
                                        }}
                                    >
                                        <ListItemButton
                                            selected={conversation.id === currentConversationId}
                                            onClick={() => {
                                                onSelect(conversation.id);
                                                onClose();
                                            }}
                                            disabled={disabled}
                                            sx={{
                                                pr: 10,
                                                '&:hover': {
                                                    bgcolor: isDark ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.06),
                                                },
                                                '&.Mui-selected': {
                                                    bgcolor: isDark ? alpha('#22B8CF', 0.20) : alpha('#15AABF', 0.15),
                                                    borderLeft: '3px solid #15AABF',
                                                    '&:hover': {
                                                        bgcolor: isDark ? alpha('#22B8CF', 0.25) : alpha('#15AABF', 0.20),
                                                    },
                                                },
                                            }}
                                        >
                                            <ListItemText
                                                primary={
                                                    <Box>
                                                        {conversation.connection && (
                                                            <Typography
                                                                variant="caption"
                                                                sx={{
                                                                    display: 'block',
                                                                    fontSize: '0.7rem',
                                                                    mb: 0.25,
                                                                    color: isDark ? '#64748B' : '#9CA3AF',
                                                                }}
                                                            >
                                                                {conversation.connection}
                                                            </Typography>
                                                        )}
                                                        <Typography
                                                            variant="body2"
                                                            noWrap
                                                            sx={{
                                                                fontWeight: conversation.id === currentConversationId ? 600 : 400,
                                                                color: isDark ? '#F1F5F9' : '#1F2937',
                                                            }}
                                                        >
                                                            {conversation.title}
                                                        </Typography>
                                                    </Box>
                                                }
                                                secondary={formatRelativeDate(conversation.updated_at)}
                                                secondaryTypographyProps={{
                                                    variant: 'caption',
                                                    sx: { color: isDark ? '#64748B' : '#9CA3AF' },
                                                }}
                                            />
                                        </ListItemButton>
                                    </ListItem>
                                ))}
                            </List>
                        )}
                    </Box>

                    {/* Delete All Button (only show if there are conversations) */}
                    {conversations.length > 0 && (
                        <>
                            <Divider sx={{ borderColor: isDark ? '#334155' : '#E5E7EB' }} />
                            <Box sx={{ p: 2 }}>
                                <Button
                                    variant="outlined"
                                    startIcon={<DeleteSweepIcon />}
                                    onClick={handleDeleteAllClick}
                                    fullWidth
                                    disabled={disabled}
                                    size="small"
                                    sx={{
                                        borderColor: isDark ? alpha('#EF4444', 0.3) : alpha('#EF4444', 0.4),
                                        color: isDark ? '#F87171' : '#DC2626',
                                        textTransform: 'none',
                                        '&:hover': {
                                            borderColor: '#EF4444',
                                            bgcolor: isDark ? alpha('#EF4444', 0.08) : alpha('#EF4444', 0.04),
                                        },
                                    }}
                                >
                                    Delete All Conversations
                                </Button>
                            </Box>
                        </>
                    )}
                </Box>
            </Drawer>

            {/* Delete Single Conversation Dialog */}
            <Dialog open={deleteDialogOpen} onClose={handleCancelDelete} PaperProps={dialogPaperProps}>
                <DialogTitle sx={{ color: isDark ? '#F1F5F9' : '#1F2937' }}>
                    Delete Conversation?
                </DialogTitle>
                <DialogContent>
                    <DialogContentText sx={{ color: isDark ? '#94A3B8' : '#6B7280' }}>
                        Are you sure you want to delete "{conversationToDelete?.title}"?
                        This action cannot be undone.
                    </DialogContentText>
                </DialogContent>
                <DialogActions sx={{ p: 2, pt: 0 }}>
                    <Button
                        onClick={handleCancelDelete}
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            textTransform: 'none',
                        }}
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleConfirmDelete}
                        variant="contained"
                        sx={{
                            bgcolor: '#EF4444',
                            color: '#FFFFFF',
                            textTransform: 'none',
                            '&:hover': {
                                bgcolor: '#DC2626',
                            },
                        }}
                    >
                        Delete
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Delete All Conversations Dialog */}
            <Dialog open={deleteAllDialogOpen} onClose={handleCancelDeleteAll} PaperProps={dialogPaperProps}>
                <DialogTitle sx={{ color: isDark ? '#F1F5F9' : '#1F2937' }}>
                    Delete All Conversations?
                </DialogTitle>
                <DialogContent>
                    <DialogContentText sx={{ color: isDark ? '#94A3B8' : '#6B7280' }}>
                        Are you sure you want to delete all {conversations.length} conversation(s)?
                        This action cannot be undone.
                    </DialogContentText>
                </DialogContent>
                <DialogActions sx={{ p: 2, pt: 0 }}>
                    <Button
                        onClick={handleCancelDeleteAll}
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            textTransform: 'none',
                        }}
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleConfirmDeleteAll}
                        variant="contained"
                        sx={{
                            bgcolor: '#EF4444',
                            color: '#FFFFFF',
                            textTransform: 'none',
                            '&:hover': {
                                bgcolor: '#DC2626',
                            },
                        }}
                    >
                        Delete All
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Rename Conversation Dialog */}
            <Dialog open={renameDialogOpen} onClose={handleCancelRename} PaperProps={dialogPaperProps}>
                <DialogTitle sx={{ color: isDark ? '#F1F5F9' : '#1F2937' }}>
                    Rename Conversation
                </DialogTitle>
                <DialogContent>
                    <TextField
                        autoFocus
                        margin="dense"
                        label="Title"
                        fullWidth
                        variant="outlined"
                        value={newTitle}
                        onChange={(e) => setNewTitle(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && newTitle.trim()) {
                                handleConfirmRename();
                            }
                        }}
                        sx={{
                            mt: 1,
                            '& .MuiOutlinedInput-root': {
                                bgcolor: isDark ? alpha('#1E293B', 0.5) : '#FFFFFF',
                                '& fieldset': {
                                    borderColor: isDark ? '#334155' : '#E5E7EB',
                                },
                                '&:hover fieldset': {
                                    borderColor: isDark ? '#475569' : '#9CA3AF',
                                },
                                '&.Mui-focused fieldset': {
                                    borderColor: '#15AABF',
                                },
                            },
                            '& .MuiInputLabel-root': {
                                color: isDark ? '#94A3B8' : '#6B7280',
                                '&.Mui-focused': {
                                    color: '#15AABF',
                                },
                            },
                            '& .MuiInputBase-input': {
                                color: isDark ? '#F1F5F9' : '#1F2937',
                            },
                        }}
                    />
                </DialogContent>
                <DialogActions sx={{ p: 2, pt: 0 }}>
                    <Button
                        onClick={handleCancelRename}
                        sx={{
                            color: isDark ? '#94A3B8' : '#6B7280',
                            textTransform: 'none',
                        }}
                    >
                        Cancel
                    </Button>
                    <Button
                        onClick={handleConfirmRename}
                        variant="contained"
                        disabled={!newTitle.trim()}
                        sx={{
                            bgcolor: '#15AABF',
                            color: '#FFFFFF',
                            textTransform: 'none',
                            '&:hover': {
                                bgcolor: '#0C8599',
                            },
                            '&.Mui-disabled': {
                                bgcolor: isDark ? '#334155' : '#E5E7EB',
                                color: isDark ? '#64748B' : '#9CA3AF',
                            },
                        }}
                    >
                        Rename
                    </Button>
                </DialogActions>
            </Dialog>
        </>
    );
};

export default ConversationPanel;
