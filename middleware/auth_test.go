package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"anshumanbiswas.com/blog/models"
)

func TestGetUserFromContext(t *testing.T) {
	t.Run("returns user when present", func(t *testing.T) {
		user := &models.User{
			UserID:   1,
			Email:    "test@example.com",
			Username: "testuser",
			Role:     models.RoleCommenter,
		}

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		result := GetUserFromContext(ctx)

		if result == nil {
			t.Fatal("Expected user, got nil")
		}

		if result.UserID != user.UserID {
			t.Errorf("UserID = %d, want %d", result.UserID, user.UserID)
		}
	})

	t.Run("returns nil when user not present", func(t *testing.T) {
		ctx := context.Background()
		result := GetUserFromContext(ctx)

		if result != nil {
			t.Errorf("Expected nil, got %+v", result)
		}
	})

	t.Run("returns nil when wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserContextKey, "not a user")
		result := GetUserFromContext(ctx)

		if result != nil {
			t.Errorf("Expected nil for wrong type, got %+v", result)
		}
	})
}

// Note: AuthenticatedUser and APIAuthMiddleware require actual service instances
// and database connections, so they are better tested through integration tests.
// Here we focus on testing the helper functions and role-based middleware.

func TestRequireRole(t *testing.T) {
	t.Run("allows admin access to admin-only resource", func(t *testing.T) {
		user := &models.User{UserID: 1, Role: models.RoleAdministrator}
		middleware := RequireRole(models.RoleAdministrator)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if !nextCalled {
			t.Error("Next handler was not called")
		}

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("blocks non-admin from admin resource", func(t *testing.T) {
		user := &models.User{UserID: 2, Role: models.RoleViewer}
		middleware := RequireRole(models.RoleAdministrator)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/admin", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if nextCalled {
			t.Error("Next handler should not be called for non-admin")
		}

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("allows editor access to editor resource", func(t *testing.T) {
		user := &models.User{UserID: 3, Role: models.RoleEditor}
		middleware := RequireRole(models.RoleEditor)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodGet, "/edit", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if !nextCalled {
			t.Error("Next handler was not called")
		}

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("blocks request without user context", func(t *testing.T) {
		middleware := RequireRole(models.RoleEditor)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		req := httptest.NewRequest(http.MethodGet, "/edit", nil)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if nextCalled {
			t.Error("Next handler should not be called without user context")
		}

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

func TestRequirePermission(t *testing.T) {
	t.Run("allows access when permission check passes", func(t *testing.T) {
		user := &models.User{UserID: 1, Role: models.RoleEditor}
		checkFunc := func(p models.UserPermissions) bool {
			return p.CanEditPosts
		}
		middleware := RequirePermission(checkFunc)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodPost, "/posts/edit", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if !nextCalled {
			t.Error("Next handler was not called")
		}

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("blocks access when permission check fails", func(t *testing.T) {
		user := &models.User{UserID: 2, Role: models.RoleViewer}
		checkFunc := func(p models.UserPermissions) bool {
			return p.CanEditPosts
		}
		middleware := RequirePermission(checkFunc)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		ctx := context.WithValue(context.Background(), UserContextKey, user)
		req := httptest.NewRequest(http.MethodPost, "/posts/edit", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if nextCalled {
			t.Error("Next handler should not be called when permission check fails")
		}

		if w.Code != http.StatusForbidden {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("blocks request without user context", func(t *testing.T) {
		checkFunc := func(p models.UserPermissions) bool {
			return true
		}
		middleware := RequirePermission(checkFunc)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
		})

		req := httptest.NewRequest(http.MethodPost, "/posts/edit", nil)
		w := httptest.NewRecorder()

		middleware(next).ServeHTTP(w, req)

		if nextCalled {
			t.Error("Next handler should not be called without user context")
		}

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}


func TestUserContextKey(t *testing.T) {
	t.Run("UserContextKey is set", func(t *testing.T) {
		if UserContextKey == "" {
			t.Error("UserContextKey should not be empty")
		}

		expected := contextKey("user")
		if UserContextKey != expected {
			t.Errorf("UserContextKey = %v, want %v", UserContextKey, expected)
		}
	})
}
