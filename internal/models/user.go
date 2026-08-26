// Package models contains the API's domain entities.
package models

import "strings"

// Role is the authorization level persisted in PostgreSQL and carried in JWTs.
type Role string

const (
	// RoleUser can authenticate but cannot access current business APIs.
	RoleUser Role = "USER"
	// RoleAdmin can administer catalogue, inventory, and uploads.
	RoleAdmin Role = "ADMIN"
)

// Valid reports whether role is a supported authorization role.
func (role Role) Valid() bool {
	return role == RoleUser || role == RoleAdmin
}

// ParseRole converts a database role into a domain role.
func ParseRole(value string) Role {
	return Role(strings.ToUpper(strings.TrimSpace(value)))
}

// Identity is the authenticated principal attached to an HTTP request.
type Identity struct {
	UserID int64
	Role   Role
}

// User is the public user profile returned after successful sign-in.
type User struct {
	ID             int64   `json:"id"`
	FullName       string  `json:"full_name"`
	Email          string  `json:"email"`
	Role           Role    `json:"role"`
	ProfilePicture *string `json:"profile_picture"`
}
