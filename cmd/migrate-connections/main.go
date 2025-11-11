/*-------------------------------------------------------------------------
 *
 * pgEdge Postgres MCP Server - Connection Migration Tool
 *
 * Copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 * This tool migrates saved connections from the old connection string
 * format to the new individual parameter format with encrypted passwords.
 *
 *-------------------------------------------------------------------------
 */

package main

import (
    "flag"
    "fmt"
    "net/url"
    "os"
    "strconv"
    "strings"
    "time"

    "pgedge-postgres-mcp/internal/auth"
    "pgedge-postgres-mcp/internal/crypto"
    "gopkg.in/yaml.v3"
)

// OldSavedConnection represents the old format with connection strings
type OldSavedConnection struct {
    Alias            string    `yaml:"alias"`
    ConnectionString string    `yaml:"connection_string"`
    MaintenanceDB    string    `yaml:"maintenance_db"`
    Description      string    `yaml:"description"`
    CreatedAt        time.Time `yaml:"created_at"`
    LastUsedAt       time.Time `yaml:"last_used_at"`
}

// OldConnectionStore represents the old format
type OldConnectionStore struct {
    Connections map[string]*OldSavedConnection `yaml:"connections"`
}

type OldPreferences struct {
    Connections *OldConnectionStore `yaml:"connections"`
}

func main() {
    prefsFile := flag.String("prefs", "", "Path to preferences file (required)")
    secretFile := flag.String("secret", "", "Path to encryption secret file (required)")
    outputFile := flag.String("output", "", "Output file (defaults to input file with .migrated extension)")
    flag.Parse()

    if *prefsFile == "" || *secretFile == "" {
        fmt.Fprintf(os.Stderr, "Usage: %s -prefs <preferences-file> -secret <secret-file> [-output <output-file>]\n", os.Args[0])
        os.Exit(1)
    }

    // Determine output file
    output := *outputFile
    if output == "" {
        output = *prefsFile + ".migrated"
    }

    fmt.Printf("Migrating connections from old to new format...\n")
    fmt.Printf("Input:  %s\n", *prefsFile)
    fmt.Printf("Secret: %s\n", *secretFile)
    fmt.Printf("Output: %s\n", output)
    fmt.Println()

    // Load encryption key
    encryptionKey, err := crypto.LoadKeyFromFile(*secretFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to load encryption key: %v\n", err)
        os.Exit(1)
    }

    // Read old preferences file
    data, err := os.ReadFile(*prefsFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to read preferences file: %v\n", err)
        os.Exit(1)
    }

    // Parse old format
    var oldPrefs OldPreferences
    if err := yaml.Unmarshal(data, &oldPrefs); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to parse preferences file: %v\n", err)
        os.Exit(1)
    }

    if oldPrefs.Connections == nil || len(oldPrefs.Connections.Connections) == 0 {
        fmt.Println("No connections found to migrate.")
        os.Exit(0)
    }

    // Convert to new format
    newStore := auth.NewSavedConnectionStore()

    for alias, oldConn := range oldPrefs.Connections.Connections {
        fmt.Printf("Migrating connection: %s\n", alias)

        newConn, err := convertConnection(oldConn, encryptionKey)
        if err != nil {
            fmt.Fprintf(os.Stderr, "ERROR: Failed to convert connection %s: %v\n", alias, err)
            continue
        }

        newStore.Connections[alias] = newConn
        fmt.Printf("  ✓ Converted: %s@%s:%d/%s\n", newConn.User, newConn.Host, newConn.Port, newConn.DBName)
    }

    // Create new preferences structure
    newPrefs := struct {
        Connections *auth.SavedConnectionStore `yaml:"connections"`
    }{
        Connections: newStore,
    }

    // Write new format
    newData, err := yaml.Marshal(newPrefs)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to marshal new preferences: %v\n", err)
        os.Exit(1)
    }

    if err := os.WriteFile(output, newData, 0600); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to write output file: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("\n✓ Migration complete!\n")
    fmt.Printf("Review the migrated file at: %s\n", output)
    fmt.Printf("If everything looks correct, replace the original file:\n")
    fmt.Printf("  mv %s %s.backup\n", *prefsFile, *prefsFile)
    fmt.Printf("  mv %s %s\n", output, *prefsFile)
}

func convertConnection(old *OldSavedConnection, encryptionKey *crypto.EncryptionKey) (*auth.SavedConnection, error) {
    // Parse connection string
    u, err := url.Parse(old.ConnectionString)
    if err != nil {
        return nil, fmt.Errorf("invalid connection string: %w", err)
    }

    newConn := &auth.SavedConnection{
        Alias:       old.Alias,
        Description: old.Description,
        CreatedAt:   old.CreatedAt,
        LastUsedAt:  old.LastUsedAt,
    }

    // Extract user and password
    if u.User != nil {
        newConn.User = u.User.Username()
        if password, hasPassword := u.User.Password(); hasPassword && password != "" {
            // Encrypt the password
            encrypted, err := encryptionKey.Encrypt(password)
            if err != nil {
                return nil, fmt.Errorf("failed to encrypt password: %w", err)
            }
            newConn.Password = encrypted
        }
    }

    // Extract host and port
    newConn.Host = u.Hostname()
    if u.Port() != "" {
        port, err := strconv.Atoi(u.Port())
        if err != nil {
            return nil, fmt.Errorf("invalid port: %w", err)
        }
        newConn.Port = port
    } else {
        newConn.Port = 5432 // Default PostgreSQL port
    }

    // Extract database name
    newConn.DBName = strings.TrimPrefix(u.Path, "/")

    // Extract query parameters (SSL, timeout, etc.)
    params := u.Query()

    if sslmode := params.Get("sslmode"); sslmode != "" {
        newConn.SSLMode = sslmode
    }
    if sslcert := params.Get("sslcert"); sslcert != "" {
        newConn.SSLCert = sslcert
    }
    if sslkey := params.Get("sslkey"); sslkey != "" {
        newConn.SSLKey = sslkey
    }
    if sslrootcert := params.Get("sslrootcert"); sslrootcert != "" {
        newConn.SSLRootCert = sslrootcert
    }
    if sslcrl := params.Get("sslcrl"); sslcrl != "" {
        newConn.SSLCRL = sslcrl
    }
    if sslpassword := params.Get("sslpassword"); sslpassword != "" {
        // Encrypt SSL password too
        encrypted, err := encryptionKey.Encrypt(sslpassword)
        if err != nil {
            return nil, fmt.Errorf("failed to encrypt SSL password: %w", err)
        }
        newConn.SSLPassword = encrypted
    }

    if connectTimeout := params.Get("connect_timeout"); connectTimeout != "" {
        timeout, err := strconv.Atoi(connectTimeout)
        if err == nil {
            newConn.ConnectTimeout = timeout
        }
    }
    if appName := params.Get("application_name"); appName != "" {
        newConn.ApplicationName = appName
    }

    return newConn, nil
}
