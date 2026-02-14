// Package main provides the Azure Resource Manager deep inspection plugin for tfclassify.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// RoleDefinition represents an Azure built-in role definition.
type RoleDefinition struct {
	// ID is the unique GUID for the role (e.g., "8e3af657-a8ff-443c-a75c-2fe8c4bcb635").
	ID string `json:"id"`
	// Name is the display name of the role (e.g., "Owner").
	Name string `json:"roleName"`
	// Description describes what the role allows.
	Description string `json:"description"`
	// RoleType indicates the role type (e.g., "BuiltInRole").
	RoleType string `json:"roleType"`
	// Permissions contains the permission blocks for the role.
	Permissions []Permission `json:"permissions"`
}

// Permission represents a permission block within a role definition.
type Permission struct {
	// Actions are the allowed control plane actions.
	Actions []string `json:"actions"`
	// NotActions are the excluded control plane actions.
	NotActions []string `json:"notActions"`
	// DataActions are the allowed data plane actions.
	DataActions []string `json:"dataActions"`
	// NotDataActions are the excluded data plane actions.
	NotDataActions []string `json:"notDataActions"`
}

// RoleDatabase provides fast lookup of Azure built-in role definitions.
type RoleDatabase struct {
	byName map[string]*RoleDefinition // keyed by lowercase roleName
	byID   map[string]*RoleDefinition // keyed by GUID (lowercase)
}

//go:embed roledata/roles.json
var builtinRolesJSON []byte

var (
	defaultDB     *RoleDatabase
	defaultDBOnce sync.Once
	defaultDBErr  error
)

// DefaultRoleDatabase returns a singleton RoleDatabase loaded from the embedded JSON.
// It is safe for concurrent access from multiple goroutines.
// Panics if the embedded data is malformed.
func DefaultRoleDatabase() *RoleDatabase {
	defaultDBOnce.Do(func() {
		defaultDB, defaultDBErr = NewRoleDatabaseFromJSON(builtinRolesJSON)
		if defaultDBErr != nil {
			panic(fmt.Sprintf("failed to initialize default role database: %v", defaultDBErr))
		}
	})
	return defaultDB
}

// NewRoleDatabaseFromJSON creates a RoleDatabase from a JSON array of role definitions.
// Returns an error if the JSON is malformed.
func NewRoleDatabaseFromJSON(data []byte) (*RoleDatabase, error) {
	var roles []RoleDefinition
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, fmt.Errorf("failed to parse role definitions: %w", err)
	}

	db := &RoleDatabase{
		byName: make(map[string]*RoleDefinition, len(roles)),
		byID:   make(map[string]*RoleDefinition, len(roles)),
	}

	for i := range roles {
		role := &roles[i]
		// Index by lowercase name
		db.byName[strings.ToLower(role.Name)] = role
		// Index by lowercase GUID
		db.byID[strings.ToLower(role.ID)] = role
	}

	return db, nil
}

// LookupByName finds a role definition by display name (case-insensitive).
// Returns the role and true if found, or nil and false if not found.
func (db *RoleDatabase) LookupByName(name string) (*RoleDefinition, bool) {
	if db == nil {
		return nil, false
	}
	role, ok := db.byName[strings.ToLower(name)]
	return role, ok
}

// LookupByID finds a role definition by ID.
// Accepts either a bare GUID (e.g., "8e3af657-a8ff-443c-a75c-2fe8c4bcb635")
// or a full ARM path (e.g., "/providers/Microsoft.Authorization/roleDefinitions/8e3af657-...").
// Returns the role and true if found, or nil and false if not found.
func (db *RoleDatabase) LookupByID(id string) (*RoleDefinition, bool) {
	if db == nil {
		return nil, false
	}

	// Extract GUID from full ARM path if present
	guid := extractGUID(id)
	role, ok := db.byID[strings.ToLower(guid)]
	return role, ok
}

// extractGUID extracts the GUID from an ARM role definition path.
// If the input is already a bare GUID, it is returned as-is.
// Example: "/providers/Microsoft.Authorization/roleDefinitions/8e3af657-..." -> "8e3af657-..."
func extractGUID(id string) string {
	// If it contains a slash, extract the last segment
	if strings.Contains(id, "/") {
		parts := strings.Split(id, "/")
		return parts[len(parts)-1]
	}
	return id
}

// RoleCount returns the number of roles in the database.
func (db *RoleDatabase) RoleCount() int {
	if db == nil {
		return 0
	}
	return len(db.byName)
}
