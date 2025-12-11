/*-------------------------------------------------------------------------
 *
 * pgEdge MCP Client - Theme Configuration
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * Theme designed to match pgEdge Cloud product aesthetics
 *
 *-------------------------------------------------------------------------
 */

import { createTheme, alpha } from '@mui/material/styles';

// pgEdge brand colors - Cyan primary to match Cloud product (Mantine cyan)
const pgedgeColors = {
    // Primary brand color - Cyan (matches pgEdge Cloud Mantine theme)
    primary: {
        main: '#15AABF',      // Mantine cyan.6
        light: '#22B8CF',     // Mantine cyan.5
        dark: '#0C8599',      // Mantine cyan.8
        contrastText: '#FFFFFF',
    },
    // Secondary - for accents
    secondary: {
        main: '#6366F1',
        light: '#818CF8',
        dark: '#4F46E5',
        contrastText: '#FFFFFF',
    },
    // Success - green for connected/success states
    success: {
        main: '#22C55E',
        light: '#4ADE80',
        dark: '#16A34A',
        contrastText: '#FFFFFF',
    },
    // Error - red for errors/failures
    error: {
        main: '#EF4444',
        light: '#F87171',
        dark: '#DC2626',
        contrastText: '#FFFFFF',
    },
    // Warning - orange/amber
    warning: {
        main: '#F59E0B',
        light: '#FBBF24',
        dark: '#D97706',
        contrastText: '#FFFFFF',
    },
    // Info - blue
    info: {
        main: '#3B82F6',
        light: '#60A5FA',
        dark: '#2563EB',
        contrastText: '#FFFFFF',
    },
};

// Light mode palette
const lightPalette = {
    mode: 'light',
    ...pgedgeColors,
    background: {
        default: '#F9FAFB',
        paper: '#FFFFFF',
    },
    text: {
        primary: '#1F2937',
        secondary: '#6B7280',
        disabled: '#9CA3AF',
    },
    divider: '#E5E7EB',
    grey: {
        50: '#F9FAFB',
        100: '#F3F4F6',
        200: '#E5E7EB',
        300: '#D1D5DB',
        400: '#9CA3AF',
        500: '#6B7280',
        600: '#4B5563',
        700: '#374151',
        800: '#1F2937',
        900: '#111827',
    },
    action: {
        active: '#6B7280',
        hover: alpha('#15AABF', 0.04),
        selected: alpha('#15AABF', 0.08),
        disabled: '#9CA3AF',
        disabledBackground: '#E5E7EB',
    },
};

// Dark mode palette
const darkPalette = {
    mode: 'dark',
    ...pgedgeColors,
    primary: {
        ...pgedgeColors.primary,
        main: '#22B8CF',      // Mantine cyan.5 for dark mode
    },
    background: {
        default: '#0F172A',
        paper: '#1E293B',
    },
    text: {
        primary: '#F1F5F9',
        secondary: '#94A3B8',
        disabled: '#64748B',
    },
    divider: '#334155',
    grey: {
        50: '#F8FAFC',
        100: '#F1F5F9',
        200: '#E2E8F0',
        300: '#CBD5E1',
        400: '#94A3B8',
        500: '#64748B',
        600: '#475569',
        700: '#334155',
        800: '#1E293B',
        900: '#0F172A',
    },
    action: {
        active: '#94A3B8',
        hover: alpha('#22B8CF', 0.08),
        selected: alpha('#22B8CF', 0.16),
        disabled: '#64748B',
        disabledBackground: '#334155',
    },
};

// Shared component overrides
const getComponents = (mode) => ({
    MuiCssBaseline: {
        styleOverrides: {
            body: {
                scrollbarColor: mode === 'dark' ? '#475569 #1E293B' : '#D1D5DB #F3F4F6',
                '&::-webkit-scrollbar, & *::-webkit-scrollbar': {
                    width: 8,
                    height: 8,
                },
                '&::-webkit-scrollbar-thumb, & *::-webkit-scrollbar-thumb': {
                    borderRadius: 4,
                    backgroundColor: mode === 'dark' ? '#475569' : '#D1D5DB',
                },
                '&::-webkit-scrollbar-track, & *::-webkit-scrollbar-track': {
                    backgroundColor: mode === 'dark' ? '#1E293B' : '#F3F4F6',
                },
            },
        },
    },
    MuiButton: {
        styleOverrides: {
            root: {
                borderRadius: 4,              // Mantine sm radius
                textTransform: 'none',
                fontWeight: 500,
                fontSize: '1rem',             // 16px (Mantine sm)
                padding: '8px 20px',
                boxShadow: 'none',
                '&:hover': {
                    boxShadow: 'none',
                },
            },
            contained: {
                '&:hover': {
                    boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
                },
            },
            containedPrimary: {
                background: '#15AABF',        // Mantine cyan.6
                '&:hover': {
                    background: '#0C8599',    // Mantine cyan.8
                },
            },
            sizeSmall: {
                padding: '6px 16px',
                fontSize: '0.875rem',         // 14px (Mantine xs)
            },
            sizeLarge: {
                padding: '12px 28px',
                fontSize: '1.125rem',         // 18px (Mantine md)
            },
        },
    },
    MuiCard: {
        styleOverrides: {
            root: {
                borderRadius: 8,              // Mantine md radius
                boxShadow: mode === 'dark'
                    ? '0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2)'
                    : '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06)',
            },
        },
    },
    MuiPaper: {
        styleOverrides: {
            root: {
                backgroundImage: 'none',
            },
            rounded: {
                borderRadius: 8,              // Mantine md radius
            },
            elevation1: {
                boxShadow: mode === 'dark'
                    ? '0 1px 3px 0 rgba(0, 0, 0, 0.3), 0 1px 2px 0 rgba(0, 0, 0, 0.2)'
                    : '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06)',
            },
            elevation2: {
                boxShadow: mode === 'dark'
                    ? '0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2)'
                    : '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
            },
        },
    },
    MuiAppBar: {
        styleOverrides: {
            root: {
                backgroundImage: 'none',
                boxShadow: mode === 'dark'
                    ? '0 1px 3px 0 rgba(0, 0, 0, 0.3)'
                    : '0 1px 3px 0 rgba(0, 0, 0, 0.1)',
            },
        },
    },
    MuiTextField: {
        styleOverrides: {
            root: {
                '& .MuiOutlinedInput-root': {
                    borderRadius: 4,          // Mantine sm radius
                    '&:hover .MuiOutlinedInput-notchedOutline': {
                        borderColor: mode === 'dark' ? '#475569' : '#9CA3AF',
                    },
                    '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
                        borderColor: '#15AABF',
                        borderWidth: 2,
                    },
                },
            },
        },
    },
    MuiOutlinedInput: {
        styleOverrides: {
            root: {
                borderRadius: 4,              // Mantine sm radius
            },
            notchedOutline: {
                borderColor: mode === 'dark' ? '#334155' : '#E5E7EB',
            },
        },
    },
    MuiChip: {
        styleOverrides: {
            root: {
                borderRadius: 6,
                fontWeight: 500,
            },
            filled: {
                '&.MuiChip-colorSuccess': {
                    backgroundColor: alpha('#22C55E', 0.15),
                    color: mode === 'dark' ? '#4ADE80' : '#16A34A',
                },
                '&.MuiChip-colorError': {
                    backgroundColor: alpha('#EF4444', 0.15),
                    color: mode === 'dark' ? '#F87171' : '#DC2626',
                },
                '&.MuiChip-colorWarning': {
                    backgroundColor: alpha('#F59E0B', 0.15),
                    color: mode === 'dark' ? '#FBBF24' : '#D97706',
                },
            },
        },
    },
    MuiIconButton: {
        styleOverrides: {
            root: {
                borderRadius: 4,              // Mantine sm radius
                '&:hover': {
                    backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                },
            },
        },
    },
    MuiTooltip: {
        styleOverrides: {
            tooltip: {
                backgroundColor: mode === 'dark' ? '#334155' : '#1F2937',
                borderRadius: 6,
                fontSize: '0.8125rem',
                padding: '8px 14px',
            },
            arrow: {
                color: mode === 'dark' ? '#334155' : '#1F2937',
            },
        },
    },
    MuiMenu: {
        styleOverrides: {
            paper: {
                borderRadius: 8,
                boxShadow: mode === 'dark'
                    ? '0 10px 15px -3px rgba(0, 0, 0, 0.3), 0 4px 6px -2px rgba(0, 0, 0, 0.2)'
                    : '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
                border: mode === 'dark' ? '1px solid #334155' : '1px solid #E5E7EB',
            },
        },
    },
    MuiMenuItem: {
        styleOverrides: {
            root: {
                borderRadius: 4,
                margin: '2px 8px',
                padding: '10px 14px',
                fontSize: '1rem',             // 16px (Mantine sm)
                '&:hover': {
                    backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.08) : alpha('#15AABF', 0.04),
                },
                '&.Mui-selected': {
                    backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.16) : alpha('#15AABF', 0.08),
                    '&:hover': {
                        backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.24) : alpha('#15AABF', 0.12),
                    },
                },
            },
        },
    },
    MuiDrawer: {
        styleOverrides: {
            paper: {
                borderRight: mode === 'dark' ? '1px solid #334155' : '1px solid #E5E7EB',
            },
        },
    },
    MuiDivider: {
        styleOverrides: {
            root: {
                borderColor: mode === 'dark' ? '#334155' : '#E5E7EB',
            },
        },
    },
    MuiAvatar: {
        styleOverrides: {
            root: {
                backgroundColor: '#15AABF',
                color: '#FFFFFF',
            },
        },
    },
    MuiSwitch: {
        styleOverrides: {
            root: {
                width: 42,
                height: 26,
                padding: 0,
            },
            switchBase: {
                padding: 0,
                margin: 2,
                transitionDuration: '300ms',
                '&.Mui-checked': {
                    transform: 'translateX(16px)',
                    color: '#fff',
                    '& + .MuiSwitch-track': {
                        backgroundColor: '#15AABF',
                        opacity: 1,
                        border: 0,
                    },
                },
            },
            thumb: {
                boxSizing: 'border-box',
                width: 22,
                height: 22,
            },
            track: {
                borderRadius: 13,
                backgroundColor: mode === 'dark' ? '#475569' : '#E5E7EB',
                opacity: 1,
            },
        },
    },
    MuiAlert: {
        styleOverrides: {
            root: {
                borderRadius: 8,
            },
            standardWarning: {
                backgroundColor: alpha('#F59E0B', 0.15),
                color: mode === 'dark' ? '#FBBF24' : '#92400E',
                '& .MuiAlert-icon': {
                    color: '#F59E0B',
                },
            },
            standardError: {
                backgroundColor: alpha('#EF4444', 0.15),
                color: mode === 'dark' ? '#F87171' : '#991B1B',
                '& .MuiAlert-icon': {
                    color: '#EF4444',
                },
            },
            standardSuccess: {
                backgroundColor: alpha('#22C55E', 0.15),
                color: mode === 'dark' ? '#4ADE80' : '#166534',
                '& .MuiAlert-icon': {
                    color: '#22C55E',
                },
            },
        },
    },
    MuiLinearProgress: {
        styleOverrides: {
            root: {
                borderRadius: 4,
                backgroundColor: mode === 'dark' ? '#334155' : '#E5E7EB',
            },
            bar: {
                borderRadius: 4,
            },
        },
    },
    MuiCircularProgress: {
        styleOverrides: {
            root: {
                color: '#15AABF',
            },
        },
    },
    MuiListItemButton: {
        styleOverrides: {
            root: {
                borderRadius: 4,              // Mantine sm radius
                '&.Mui-selected': {
                    backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.20) : alpha('#15AABF', 0.15),
                    '&:hover': {
                        backgroundColor: mode === 'dark' ? alpha('#22B8CF', 0.25) : alpha('#15AABF', 0.20),
                    },
                },
            },
        },
    },
});

// Typography configuration - Larger fonts to match Cloud (Mantine defaults)
// Mantine: xs=14px, sm=16px, md=18px, lg=20px, xl=22px
const typography = {
    fontFamily: '"Inter", "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, sans-serif',
    fontSize: 16,                     // Base font size (Mantine sm)
    htmlFontSize: 16,
    h1: {
        fontWeight: 700,
        fontSize: '2.75rem',          // 44px
        lineHeight: 1.2,
        letterSpacing: '-0.02em',
    },
    h2: {
        fontWeight: 700,
        fontSize: '2.25rem',          // 36px
        lineHeight: 1.3,
        letterSpacing: '-0.01em',
    },
    h3: {
        fontWeight: 600,
        fontSize: '1.875rem',         // 30px
        lineHeight: 1.4,
    },
    h4: {
        fontWeight: 600,
        fontSize: '1.5rem',           // 24px
        lineHeight: 1.4,
    },
    h5: {
        fontWeight: 600,
        fontSize: '1.375rem',         // 22px (Mantine xl)
        lineHeight: 1.5,
    },
    h6: {
        fontWeight: 600,
        fontSize: '1.25rem',          // 20px (Mantine lg)
        lineHeight: 1.5,
    },
    subtitle1: {
        fontWeight: 500,
        fontSize: '1.125rem',         // 18px (Mantine md)
        lineHeight: 1.5,
    },
    subtitle2: {
        fontWeight: 500,
        fontSize: '1rem',             // 16px (Mantine sm)
        lineHeight: 1.5,
    },
    body1: {
        fontSize: '1.125rem',         // 18px (Mantine md)
        lineHeight: 1.6,
    },
    body2: {
        fontSize: '1rem',             // 16px (Mantine sm)
        lineHeight: 1.6,
    },
    button: {
        fontWeight: 500,
        fontSize: '1rem',             // 16px (Mantine sm)
        letterSpacing: '0.02em',
    },
    caption: {
        fontSize: '0.875rem',         // 14px (Mantine xs)
        lineHeight: 1.5,
    },
    overline: {
        fontSize: '0.875rem',         // 14px (Mantine xs)
        fontWeight: 600,
        letterSpacing: '0.08em',
        textTransform: 'uppercase',
    },
};

// Shape configuration
const shape = {
    borderRadius: 8,
};

// Create theme function
export const createPgedgeTheme = (mode = 'light') => {
    const palette = mode === 'dark' ? darkPalette : lightPalette;

    return createTheme({
        palette,
        typography,
        shape,
        components: getComponents(mode),
    });
};

// Export light theme for login (always light)
export const loginTheme = createPgedgeTheme('light');

// Default export
export default createPgedgeTheme;
