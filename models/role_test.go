package models

import (
	"testing"
)

// TestRoleMatrix tests the permissions matrix for different roles
func TestRoleMatrix(t *testing.T) {
	// Commenter
	c := GetPermissions(RoleCommenter)
	if !c.CanComment || c.CanEditPosts || c.CanManageUsers || c.CanViewAdmin {
		t.Fatalf("commenter perms incorrect: %+v", c)
	}

	// Viewer
	v := GetPermissions(RoleViewer)
	if !v.CanComment || v.CanEditPosts || v.CanManageUsers || v.CanViewAdmin {
		t.Fatalf("viewer perms incorrect: %+v", v)
	}

	// Editor
	e := GetPermissions(RoleEditor)
	if !e.CanEditPosts || e.CanManageUsers || !e.CanViewAdmin || !e.CanEditSlides {
		t.Fatalf("editor perms incorrect: %+v", e)
	}

	// Admin
	a := GetPermissions(RoleAdministrator)
	if !(a.CanComment && a.CanEditPosts && a.CanManageUsers && a.CanViewUnpublished && a.CanViewAdmin && a.CanEditSlides) {
		t.Fatalf("admin perms incorrect: %+v", a)
	}
}

// TestIsAdmin tests the IsAdmin function
func TestIsAdmin(t *testing.T) {
	if !IsAdmin(RoleAdministrator) {
		t.Fatalf("expected admin role to be admin")
	}
	if IsAdmin(RoleCommenter) || IsAdmin(RoleEditor) || IsAdmin(RoleViewer) {
		t.Fatalf("only administrator should be admin")
	}
}

// TestPermissionsMatrix tests the detailed permissions for each role
func TestPermissionsMatrix(t *testing.T) {
	// Commenter
	p := GetPermissions(RoleCommenter)
	if p.CanEditPosts || p.CanManageUsers || p.CanViewUnpublished {
		t.Fatalf("commenter permissions too broad: %+v", p)
	}

	// Editor
	p = GetPermissions(RoleEditor)
	if !p.CanEditPosts || p.CanManageUsers {
		t.Fatalf("editor permissions incorrect: %+v", p)
	}

	// Admin
	p = GetPermissions(RoleAdministrator)
	if !(p.CanEditPosts && p.CanManageUsers && p.CanViewUnpublished) {
		t.Fatalf("admin permissions incorrect: %+v", p)
	}
}

// TestCanEditPosts tests the CanEditPosts helper function
func TestCanEditPosts(t *testing.T) {
	if CanEditPosts(RoleCommenter) {
		t.Error("Commenter should not be able to edit posts")
	}
	if CanEditPosts(RoleViewer) {
		t.Error("Viewer should not be able to edit posts")
	}
	if !CanEditPosts(RoleEditor) {
		t.Error("Editor should be able to edit posts")
	}
	if !CanEditPosts(RoleAdministrator) {
		t.Error("Administrator should be able to edit posts")
	}
}

// TestCanViewUnpublished tests the CanViewUnpublished helper function
func TestCanViewUnpublished(t *testing.T) {
	if CanViewUnpublished(RoleCommenter) {
		t.Error("Commenter should not view unpublished posts")
	}
	if !CanViewUnpublished(RoleViewer) {
		t.Error("Viewer should view unpublished posts")
	}
	if !CanViewUnpublished(RoleEditor) {
		t.Error("Editor should view unpublished posts")
	}
	if !CanViewUnpublished(RoleAdministrator) {
		t.Error("Administrator should view unpublished posts")
	}
}

// TestCanEditSlides tests the CanEditSlides helper function
func TestCanEditSlides(t *testing.T) {
	if CanEditSlides(RoleCommenter) {
		t.Error("Commenter should not be able to edit slides")
	}
	if CanEditSlides(RoleViewer) {
		t.Error("Viewer should not be able to edit slides")
	}
	if !CanEditSlides(RoleEditor) {
		t.Error("Editor should be able to edit slides")
	}
	if !CanEditSlides(RoleAdministrator) {
		t.Error("Administrator should be able to edit slides")
	}
}

// TestCanViewAdminPanel tests the CanViewAdminPanel helper function
func TestCanViewAdminPanel(t *testing.T) {
	if CanViewAdminPanel(RoleCommenter) {
		t.Error("Commenter should not view admin panel")
	}
	if CanViewAdminPanel(RoleViewer) {
		t.Error("Viewer should not view admin panel")
	}
	if !CanViewAdminPanel(RoleEditor) {
		t.Error("Editor should view admin panel")
	}
	if !CanViewAdminPanel(RoleAdministrator) {
		t.Error("Administrator should view admin panel")
	}
}

// TestDefaultRolePermissions tests that unknown roles default to commenter permissions
func TestDefaultRolePermissions(t *testing.T) {
	unknownRole := 999
	p := GetPermissions(unknownRole)
	commenterPerms := GetPermissions(RoleCommenter)

	if p.CanEditPosts != commenterPerms.CanEditPosts ||
		p.CanManageUsers != commenterPerms.CanManageUsers ||
		p.CanViewUnpublished != commenterPerms.CanViewUnpublished {
		t.Errorf("Unknown role should default to commenter permissions, got: %+v", p)
	}
}

// TestRoleConstants verifies the role constant values
func TestRoleConstants(t *testing.T) {
	if RoleCommenter != 1 {
		t.Errorf("RoleCommenter should be 1, got %d", RoleCommenter)
	}
	if RoleAdministrator != 2 {
		t.Errorf("RoleAdministrator should be 2, got %d", RoleAdministrator)
	}
	if RoleEditor != 3 {
		t.Errorf("RoleEditor should be 3, got %d", RoleEditor)
	}
	if RoleViewer != 4 {
		t.Errorf("RoleViewer should be 4, got %d", RoleViewer)
	}
}
