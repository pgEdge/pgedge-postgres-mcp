/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Copyright (c) 2025 - 2026, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package auth

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain hashes passwords at the minimum cost, since nothing here
// depends on the cost and the race detector makes cost 12 very slow.
func TestMain(m *testing.M) {
	hashCost = bcrypt.MinCost
	os.Exit(m.Run())
}
