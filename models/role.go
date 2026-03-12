package models

import (
	"database/sql"
	"fmt"
)

// Role constants for the 4 different user roles
const (
	RoleCommenter     = 1
	RoleAdministrator = 2  
	RoleEditor        = 3
	RoleViewer        = 4
)

type Role struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type RoleService struct {
	DB *sql.DB
}

// GetAllRoles returns all available roles
func (rs *RoleService) GetAllRoles() ([]*Role, error) {
	query := `SELECT role_id, role_name FROM roles ORDER BY role_id`
	
	rows, err := rs.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}
	defer rows.Close()
	
	var roles []*Role
	for rows.Next() {
		role := &Role{}
		err := rows.Scan(&role.ID, &role.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, role)
	}
	
	return roles, nil
}

// GetRoleName returns the name of a role by ID
func (rs *RoleService) GetRoleName(roleID int) (string, error) {
	query := `SELECT role_name FROM roles WHERE role_id = $1`
	
	var roleName string
	err := rs.DB.QueryRow(query, roleID).Scan(&roleName)
	if err != nil {
		return "", fmt.Errorf("failed to get role name: %w", err)
	}
	
	return roleName, nil
}

// UserPermissions defines what each role can do
type UserPermissions struct {
	CanComment          bool
	CanViewUnpublished  bool
	CanEditPosts        bool
	CanEditSlides       bool
	CanManageAllPosts   bool
	CanManageUsers      bool
	CanViewAdmin        bool
}

// GetPermissions returns the permissions for a given role
func GetPermissions(roleID int) UserPermissions {
	switch roleID {
	case RoleCommenter:
		return UserPermissions{
			CanComment:          true,
			CanViewUnpublished:  false,
			CanEditPosts:        false,
			CanEditSlides:       false,
			CanManageAllPosts:   false,
			CanManageUsers:      false,
			CanViewAdmin:        false,
		}
	case RoleAdministrator:
		return UserPermissions{
			CanComment:          true,
			CanViewUnpublished:  true,
			CanEditPosts:        true,
			CanEditSlides:       true,
			CanManageAllPosts:   true,
			CanManageUsers:      true,
			CanViewAdmin:        true,
		}
	case RoleEditor:
		return UserPermissions{
			CanComment:          true,
			CanViewUnpublished:  true,
			CanEditPosts:        true,
			CanEditSlides:       true,
			CanManageAllPosts:   false,
			CanManageUsers:      false,
			CanViewAdmin:        true,
		}
	case RoleViewer:
		return UserPermissions{
			CanComment:          true,
			CanViewUnpublished:  true,
			CanEditPosts:        false,
			CanEditSlides:       false,
			CanManageAllPosts:   false,
			CanManageUsers:      false,
			CanViewAdmin:        false,
		}
	default:
		// Default to commenter permissions for unknown roles
		return GetPermissions(RoleCommenter)
	}
}

// IsAdmin checks if a role has admin privileges
func IsAdmin(roleID int) bool {
	return roleID == RoleAdministrator
}

// CanEditPosts checks if a role can edit posts
func CanEditPosts(roleID int) bool {
	permissions := GetPermissions(roleID)
	return permissions.CanEditPosts
}

// CanEditSlides checks if a role can edit slides
func CanEditSlides(roleID int) bool {
	permissions := GetPermissions(roleID)
	return permissions.CanEditSlides
}

// CanViewAdmin checks if a role can view the admin panel
func CanViewAdminPanel(roleID int) bool {
	permissions := GetPermissions(roleID)
	return permissions.CanViewAdmin
}

// CanViewUnpublished checks if a role can view unpublished posts
func CanViewUnpublished(roleID int) bool {
	permissions := GetPermissions(roleID)
	return permissions.CanViewUnpublished
}