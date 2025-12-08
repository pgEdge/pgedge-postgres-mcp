/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Conversation Panel
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
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
    ListItemSecondaryAction,
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
                            borderBottom: 1,
                            borderColor: 'divider',
                        }}
                    >
                        <Typography variant="h6" component="h2">
                            Conversations
                        </Typography>
                        <IconButton onClick={onClose} aria-label="close panel">
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
                        >
                            New Conversation
                        </Button>
                    </Box>

                    <Divider />

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
                                <CircularProgress size={32} />
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
                                <ChatIcon sx={{ fontSize: 48, color: 'text.secondary', mb: 2 }} />
                                <Typography variant="body1" color="text.secondary">
                                    No conversations yet
                                </Typography>
                                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
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
                                            sx={{ pr: 10 }}
                                        >
                                            <ListItemText
                                                primary={
                                                    <Box>
                                                        {conversation.connection && (
                                                            <Typography
                                                                variant="caption"
                                                                color="text.secondary"
                                                                sx={{ display: 'block', fontSize: '0.7rem', mb: 0.25 }}
                                                            >
                                                                {conversation.connection}
                                                            </Typography>
                                                        )}
                                                        <Typography
                                                            variant="body2"
                                                            noWrap
                                                            sx={{ fontWeight: conversation.id === currentConversationId ? 600 : 400 }}
                                                        >
                                                            {conversation.title}
                                                        </Typography>
                                                    </Box>
                                                }
                                                secondary={formatRelativeDate(conversation.updated_at)}
                                                secondaryTypographyProps={{
                                                    variant: 'caption',
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
                            <Divider />
                            <Box sx={{ p: 2 }}>
                                <Button
                                    variant="outlined"
                                    color="error"
                                    startIcon={<DeleteSweepIcon />}
                                    onClick={handleDeleteAllClick}
                                    fullWidth
                                    disabled={disabled}
                                    size="small"
                                >
                                    Delete All Conversations
                                </Button>
                            </Box>
                        </>
                    )}
                </Box>
            </Drawer>

            {/* Delete Single Conversation Dialog */}
            <Dialog open={deleteDialogOpen} onClose={handleCancelDelete}>
                <DialogTitle>Delete Conversation?</DialogTitle>
                <DialogContent>
                    <DialogContentText>
                        Are you sure you want to delete "{conversationToDelete?.title}"?
                        This action cannot be undone.
                    </DialogContentText>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleCancelDelete}>Cancel</Button>
                    <Button onClick={handleConfirmDelete} color="error" variant="contained">
                        Delete
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Delete All Conversations Dialog */}
            <Dialog open={deleteAllDialogOpen} onClose={handleCancelDeleteAll}>
                <DialogTitle>Delete All Conversations?</DialogTitle>
                <DialogContent>
                    <DialogContentText>
                        Are you sure you want to delete all {conversations.length} conversation(s)?
                        This action cannot be undone.
                    </DialogContentText>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleCancelDeleteAll}>Cancel</Button>
                    <Button onClick={handleConfirmDeleteAll} color="error" variant="contained">
                        Delete All
                    </Button>
                </DialogActions>
            </Dialog>

            {/* Rename Conversation Dialog */}
            <Dialog open={renameDialogOpen} onClose={handleCancelRename}>
                <DialogTitle>Rename Conversation</DialogTitle>
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
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleCancelRename}>Cancel</Button>
                    <Button
                        onClick={handleConfirmRename}
                        color="primary"
                        variant="contained"
                        disabled={!newTitle.trim()}
                    >
                        Rename
                    </Button>
                </DialogActions>
            </Dialog>
        </>
    );
};

export default ConversationPanel;
