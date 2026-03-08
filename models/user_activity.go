package models

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserActivity represents a single login/action event for a user.
type UserActivity struct {
	ID           int
	UserID       int
	ActivityType string  // "login", "failed_login", "password_change", "role_change"
	IPAddress    *string
	UserAgent    *string
	CreatedAt    time.Time
}

// UserWithStats augments a User with aggregated login/activity data.
type UserWithStats struct {
	UserID           int
	Username         string
	FullName         string
	Email            string
	Role             int
	RoleName         string
	RegistrationDate string
	AuthProvider     string
	LoginCount       int
	FailedAttempts   int
	LastLoginAt      *string
	LastLoginIP      *string
}

// UserActivityService manages the user_activity table and related admin ops.
type UserActivityService struct {
	DB *sql.DB
}

// Log inserts an activity record. Errors are silently dropped so callers
// never fail because of audit-logging.
func (s *UserActivityService) Log(userID int, activityType, ipAddress, userAgent string) {
	_, _ = s.DB.Exec(
		`INSERT INTO user_activity (user_id, activity_type, ip_address, user_agent, created_at)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), $5)`,
		userID, activityType, ipAddress, userAgent, time.Now().UTC(),
	)
}

// GetUserActivity returns the most recent activity records for a user.
func (s *UserActivityService) GetUserActivity(userID, limit int) ([]UserActivity, error) {
	rows, err := s.DB.Query(
		`SELECT id, user_id, activity_type, ip_address, user_agent, created_at
		 FROM user_activity WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get user activity: %w", err)
	}
	defer rows.Close()

	var activities []UserActivity
	for rows.Next() {
		var a UserActivity
		if err := rows.Scan(&a.ID, &a.UserID, &a.ActivityType, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user activity: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, rows.Err()
}

// GetUsersWithStats returns all users joined with their login counts and last-login info.
func (s *UserActivityService) GetUsersWithStats() ([]UserWithStats, error) {
	rows, err := s.DB.Query(`
		SELECT
			u.user_id,
			u.username,
			COALESCE(u.full_name, '') AS full_name,
			u.email,
			u.role_id,
			COALESCE(r.role_name, '') AS role_name,
			COALESCE(TO_CHAR(u.registration_date, 'YYYY-MM-DD'), '') AS registration_date,
			COALESCE(u.auth_provider, 'password') AS auth_provider,
			COUNT(CASE WHEN a.activity_type = 'login' THEN 1 END)        AS login_count,
			COUNT(CASE WHEN a.activity_type = 'failed_login' THEN 1 END) AS failed_attempts,
			MAX(CASE WHEN a.activity_type = 'login' THEN a.created_at END) AS last_login_at,
			(SELECT ip_address FROM user_activity
			 WHERE user_id = u.user_id AND activity_type = 'login'
			 ORDER BY created_at DESC LIMIT 1) AS last_login_ip
		FROM Users u
		LEFT JOIN roles r ON u.role_id = r.role_id
		LEFT JOIN user_activity a ON u.user_id = a.user_id
		GROUP BY u.user_id, u.username, u.full_name, u.email, u.role_id,
		         r.role_name, u.registration_date, u.auth_provider
		ORDER BY u.registration_date DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("get users with stats: %w", err)
	}
	defer rows.Close()

	var users []UserWithStats
	for rows.Next() {
		var u UserWithStats
		var lastLoginAt sql.NullTime
		var lastLoginIP sql.NullString
		if err := rows.Scan(
			&u.UserID, &u.Username, &u.FullName, &u.Email, &u.Role,
			&u.RoleName, &u.RegistrationDate, &u.AuthProvider,
			&u.LoginCount, &u.FailedAttempts, &lastLoginAt, &lastLoginIP,
		); err != nil {
			return nil, fmt.Errorf("scan user with stats: %w", err)
		}
		if lastLoginAt.Valid {
			t := lastLoginAt.Time.Format("2006-01-02 15:04")
			u.LastLoginAt = &t
		}
		if lastLoginIP.Valid {
			u.LastLoginIP = &lastLoginIP.String
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUserRole changes the role of a user.
func (s *UserActivityService) UpdateUserRole(userID, newRole int) error {
	_, err := s.DB.Exec("UPDATE Users SET role_id = $1 WHERE user_id = $2", newRole, userID)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	return nil
}

// AdminResetPassword sets a new bcrypt password for the target user.
func (s *UserActivityService) AdminResetPassword(userID int, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = s.DB.Exec("UPDATE Users SET password = $1 WHERE user_id = $2", string(hashed), userID)
	if err != nil {
		return fmt.Errorf("admin reset password: %w", err)
	}
	return nil
}
