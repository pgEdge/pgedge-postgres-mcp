/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package api

import (
	"encoding/json"
	"net/http"

	"pgedge-postgres-mcp/internal/auth"
	"pgedge-postgres-mcp/internal/config"
	"pgedge-postgres-mcp/internal/database"
)

// DatabaseInfo represents a database in the API response
type DatabaseInfo struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	SSLMode  string `json:"sslmode"`
}

// ListDatabasesResponse is the response for GET /api/databases
type ListDatabasesResponse struct {
	Databases []DatabaseInfo `json:"databases"`
	Current   string         `json:"current"`
}

// SelectDatabaseRequest is the request body for POST /api/databases/select
type SelectDatabaseRequest struct {
	Name string `json:"name"`
}

// SelectDatabaseResponse is the response for POST /api/databases/select
type SelectDatabaseResponse struct {
	Success bool   `json:"success"`
	Current string `json:"current,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DatabaseHandler handles database listing and selection API endpoints
type DatabaseHandler struct {
	clientManager *database.ClientManager
	accessChecker *auth.DatabaseAccessChecker
	isSTDIO       bool
	authEnabled   bool
}

// NewDatabaseHandler creates a new database handler
func NewDatabaseHandler(
	clientManager *database.ClientManager,
	accessChecker *auth.DatabaseAccessChecker,
	isSTDIO, authEnabled bool,
) *DatabaseHandler {
	return &DatabaseHandler{
		clientManager: clientManager,
		accessChecker: accessChecker,
		isSTDIO:       isSTDIO,
		authEnabled:   authEnabled,
	}
}

// HandleListDatabases handles GET /api/databases
func (h *DatabaseHandler) HandleListDatabases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	tokenHash := auth.GetTokenHashFromContext(ctx)

	// Get all configured databases
	allConfigs := h.clientManager.GetDatabaseConfigs()

	// Filter by access control
	var accessibleConfigs []config.NamedDatabaseConfig
	if h.accessChecker != nil {
		accessibleConfigs = h.accessChecker.GetAccessibleDatabases(ctx, allConfigs)
	} else {
		// No access checker - return all (STDIO or no-auth mode)
		accessibleConfigs = allConfigs
	}

	// Build response
	databases := make([]DatabaseInfo, 0, len(accessibleConfigs))
	for i := range accessibleConfigs {
		cfg := &accessibleConfigs[i]
		databases = append(databases, DatabaseInfo{
			Name:     cfg.Name,
			Host:     cfg.Host,
			Port:     cfg.Port,
			Database: cfg.Database,
			User:     cfg.User,
			SSLMode:  cfg.SSLMode,
		})
	}

	// Get current database for this token
	current := h.clientManager.GetCurrentDatabase(tokenHash)
	if current == "" {
		current = h.clientManager.GetDefaultDatabaseName()
	}

	response := ListDatabasesResponse{
		Databases: databases,
		Current:   current,
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // Error would only occur if connection is closed
	json.NewEncoder(w).Encode(response)
}

// HandleSelectDatabase handles POST /api/databases/select
func (h *DatabaseHandler) HandleSelectDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	tokenHash := auth.GetTokenHashFromContext(ctx)

	// Parse request body
	var req SelectDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		//nolint:errcheck // Error would only occur if connection is closed
		json.NewEncoder(w).Encode(SelectDatabaseResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		//nolint:errcheck // Error would only occur if connection is closed
		json.NewEncoder(w).Encode(SelectDatabaseResponse{
			Success: false,
			Error:   "Database name is required",
		})
		return
	}

	// Check if database exists
	dbConfig := h.clientManager.GetDatabaseConfig(req.Name)
	if dbConfig == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck // Error would only occur if connection is closed
		json.NewEncoder(w).Encode(SelectDatabaseResponse{
			Success: false,
			Error:   "Database not found",
		})
		return
	}

	// Check access
	if h.accessChecker != nil {
		// For API tokens, check if they're bound to a different database
		if auth.IsAPITokenFromContext(ctx) {
			boundDB := h.accessChecker.GetBoundDatabase(ctx)
			if boundDB != "" && boundDB != req.Name {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				//nolint:errcheck // Error would only occur if connection is closed
				json.NewEncoder(w).Encode(SelectDatabaseResponse{
					Success: false,
					Error:   "API token is bound to a different database",
				})
				return
			}
		}

		// Check if user has access to this database
		if !h.accessChecker.CanAccessDatabase(ctx, dbConfig) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			//nolint:errcheck // Error would only occur if connection is closed
			json.NewEncoder(w).Encode(SelectDatabaseResponse{
				Success: false,
				Error:   "Access denied to this database",
			})
			return
		}
	}

	// Set current database for this token
	if tokenHash != "" {
		if err := h.clientManager.SetCurrentDatabase(tokenHash, req.Name); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			//nolint:errcheck // Error would only occur if connection is closed
			json.NewEncoder(w).Encode(SelectDatabaseResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // Error would only occur if connection is closed
	json.NewEncoder(w).Encode(SelectDatabaseResponse{
		Success: true,
		Current: req.Name,
		Message: "Database selected successfully",
	})
}
