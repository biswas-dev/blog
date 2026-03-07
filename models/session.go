package models

import (
	"database/sql"
	"fmt"
	"strings"

	"anshumanbiswas.com/blog/rand"
	"golang.org/x/crypto/bcrypt"
)

type Session struct {
	ID     int
	UserID int
	// Token is only set when creating a new session. When looking up a session
	// this will be left empty, as we only store the hash of a session token
	// in our database and we cannot reverse it into a raw token.
	Token     string
	TokenHash string
	CreatedAt string
}

type SessionService struct {
	DB *sql.DB
}

// Create will create a new session for the user provided. The session token
// will be returned as the Token field on the Session type, but only the hashed
// session token is stored in the database.
func (ss *SessionService) Create(userID int) (*Session, error) {
	// TODO: Create the session token
	token, err := rand.SessionToken()
	if err != nil {
		return nil, fmt.Errorf("create session token: %w", err)
	}

	hashedToken, err := ss.GenerateHashedToken(token)

	if err != nil {
		return nil, fmt.Errorf("create token, generate hashed token: %w", err)
	}

	id, err := ss.GetSession(userID)

	if err != nil {
		return nil, fmt.Errorf("create token, fetch existing token: %w", err)
	}

	row := ss.DB.QueryRow("")

	if id != 0 {
		row = ss.DB.QueryRow(`
		UPDATE sessions set token_hash = $1 where ID = $2
		RETURNING id`, hashedToken, id)
	} else {
		row = ss.DB.QueryRow(`
			INSERT INTO sessions (user_id, token_hash)
			VALUES ($1, $2) RETURNING id`, userID, hashedToken)
	}

	session := Session{
		Token:     token,
		TokenHash: hashedToken,
	}

	err = row.Scan(&session.ID)

	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	return &session, nil
}

func (ss *SessionService) GetSession(userID int) (int, error) {
	var id int

	query := `SELECT id FROM sessions WHERE user_id = $1`
	err := ss.DB.QueryRow(query, userID).Scan(&id)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("error fetching session: %w", err)
	}
	return id, nil
}

func (ss *SessionService) User(token string, email string) (*User, error) {

	email = strings.ToLower(email)

	user := User{}
	session := Session{
		Token: token,
	}

	row := ss.DB.QueryRow(`
		SELECT s.id, s.token_hash, u.user_id, u.username, u.full_name, u.role_id
		FROM users AS u
		INNER JOIN sessions AS s ON u.user_id = s.user_id
		WHERE u.email = $1`, email)

	var dbUserID int
	err := row.Scan(&session.ID, &session.TokenHash, &dbUserID, &user.Username, &user.FullName, &user.Role)
	if err != nil {
		return nil, fmt.Errorf("session, email incorrect: %w", err)
	}

	if err = bcrypt.CompareHashAndPassword([]byte(session.TokenHash), []byte(token)); err != nil {
		return nil, fmt.Errorf("authenticate session: %w", err)
	}

	user.Email = email
	user.UserID = dbUserID
	return &user, nil
}

func (ss *SessionService) Logout(email string) {

	email = strings.ToLower(email)

	ss.DB.QueryRow(`DELETE FROM sessions WHERE user_id IN (SELECT user_id FROM users WHERE email = $1)`, email)

}

func (ss *SessionService) GenerateHashedToken(token string) (string, error) {
	hashedTokenBytes, err := bcrypt.GenerateFromPassword(
		[]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}

	return string(hashedTokenBytes), nil
}
