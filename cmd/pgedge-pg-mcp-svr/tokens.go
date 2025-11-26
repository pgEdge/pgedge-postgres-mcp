/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"pgedge-postgres-mcp/internal/auth"
)

// addTokenCommand handles the add-token command
// database parameter specifies the database this token is bound to (empty = prompt or use first)
// availableDatabases is the list of configured database names for interactive selection
func addTokenCommand(tokenFile, annotation, database string, expiresIn time.Duration, availableDatabases []string) error {
	// Load or create token store
	var store *auth.TokenStore
	var err error

	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		store = auth.InitializeTokenStore()
		fmt.Fprintf(os.Stderr, "Creating new token file: %s\n", tokenFile)
	} else {
		store, err = auth.LoadTokenStore(tokenFile)
		if err != nil {
			return fmt.Errorf("failed to load token file: %w", err)
		}
	}

	// Generate token
	token, err := auth.GenerateToken()
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Hash token
	hash := auth.HashToken(token)

	reader := bufio.NewReader(os.Stdin)

	// Prompt for annotation if not provided
	if annotation == "" {
		fmt.Print("Enter annotation/note for this token (optional): ")
		if input, err := reader.ReadString('\n'); err == nil {
			annotation = strings.TrimSpace(input)
		}
	}

	// Prompt for database binding if not provided
	if database == "" && len(availableDatabases) > 0 {
		fmt.Println("\nAvailable databases:")
		for i, db := range availableDatabases {
			if i == 0 {
				fmt.Printf("  %d. %s (default)\n", i+1, db)
			} else {
				fmt.Printf("  %d. %s\n", i+1, db)
			}
		}
		fmt.Print("Bind token to database (enter name or number, blank for default): ")
		if input, err := reader.ReadString('\n'); err == nil {
			input = strings.TrimSpace(input)
			if input != "" {
				// Check if it's a number
				var num int
				if _, err := fmt.Sscanf(input, "%d", &num); err == nil {
					if num >= 1 && num <= len(availableDatabases) {
						database = availableDatabases[num-1]
					} else {
						return fmt.Errorf("invalid database number: %d (must be 1-%d)", num, len(availableDatabases))
					}
				} else {
					// Check if it's a valid database name
					found := false
					for _, db := range availableDatabases {
						if db == input {
							database = input
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("unknown database: %s", input)
					}
				}
			}
		}
	}

	// Calculate expiry
	var expiresAt *time.Time
	if expiresIn > 0 {
		expiry := time.Now().Add(expiresIn)
		expiresAt = &expiry
	} else if expiresIn == 0 {
		// Prompt for expiry
		fmt.Print("Enter expiry duration (e.g., '30d', '1y', or 'never'): ")
		input := ""
		if userInput, err := reader.ReadString('\n'); err == nil {
			input = strings.TrimSpace(userInput)
		}

		if input != "" && input != "never" {
			duration, err := parseDuration(input)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			expiry := time.Now().Add(duration)
			expiresAt = &expiry
		}
	}

	// Generate unique ID
	tokenID := fmt.Sprintf("token-%d", time.Now().Unix())

	// Add token to store
	if err := store.AddToken(tokenID, hash, annotation, expiresAt, database); err != nil {
		return fmt.Errorf("failed to add token: %w", err)
	}

	// Save token store
	if err := auth.SaveTokenStore(tokenFile, store); err != nil {
		return fmt.Errorf("failed to save token file: %w", err)
	}

	// Display results
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Token created successfully!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nToken: %s\n", token)
	fmt.Printf("Hash:  %s\n", hash[:16]+"...")
	fmt.Printf("ID:    %s\n", tokenID)
	if annotation != "" {
		fmt.Printf("Note:  %s\n", annotation)
	}
	if database != "" {
		fmt.Printf("Database: %s\n", database)
	} else {
		fmt.Println("Database: (first configured)")
	}
	if expiresAt != nil {
		fmt.Printf("Expires: %s\n", expiresAt.Format(time.RFC3339))
	} else {
		fmt.Println("Expires: Never")
	}
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\nIMPORTANT: Save this token securely - it will not be shown again!")
	fmt.Println("Use it in API requests with: Authorization: Bearer <token>")
	fmt.Println(strings.Repeat("=", 70) + "\n")

	return nil
}

// removeTokenCommand handles the remove-token command
func removeTokenCommand(tokenFile, identifier string) error {
	// Load token store
	store, err := auth.LoadTokenStore(tokenFile)
	if err != nil {
		return fmt.Errorf("failed to load token file: %w", err)
	}

	// Remove token
	removed, err := store.RemoveToken(identifier)
	if err != nil {
		return fmt.Errorf("failed to remove token: %w", err)
	}

	if !removed {
		return fmt.Errorf("token not found: %s", identifier)
	}

	// Save token store
	if err := auth.SaveTokenStore(tokenFile, store); err != nil {
		return fmt.Errorf("failed to save token file: %w", err)
	}

	fmt.Printf("Token removed successfully: %s\n", identifier)
	return nil
}

// listTokensCommand handles the list-tokens command
func listTokensCommand(tokenFile string) error {
	// Load token store
	store, err := auth.LoadTokenStore(tokenFile)
	if err != nil {
		return fmt.Errorf("failed to load token file: %w", err)
	}

	tokens := store.ListTokens()
	if len(tokens) == 0 {
		fmt.Println("No tokens found.")
		return nil
	}

	fmt.Println("\nAPI Tokens:")
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-20s %-14s %-15s %-18s %-10s %s\n", "ID", "Hash Prefix", "Database", "Expires", "Status", "Annotation")
	fmt.Println(strings.Repeat("-", 100))

	for _, token := range tokens {
		status := "Active"
		if token.Expired {
			status = "EXPIRED"
		}

		expiryStr := "Never"
		if token.ExpiresAt != nil {
			expiryStr = token.ExpiresAt.Format("2006-01-02 15:04")
		}

		database := token.Database
		if database == "" {
			database = "(default)"
		} else if len(database) > 13 {
			database = database[:10] + "..."
		}

		annotation := token.Annotation
		if len(annotation) > 20 {
			annotation = annotation[:17] + "..."
		}

		fmt.Printf("%-20s %-14s %-15s %-18s %-10s %s\n",
			token.ID,
			token.HashPrefix,
			database,
			expiryStr,
			status,
			annotation)
	}
	fmt.Println(strings.Repeat("=", 100) + "\n")

	return nil
}

// parseDuration parses durations like "30d", "1y", "2w", "12h"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	// Get the numeric part and unit
	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid number in duration: %w", err)
	}

	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(num) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %c (use h, d, w, m, or y)", unit)
	}
}
