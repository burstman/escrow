package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"escrow/internal/model"
	"escrow/web/templates"
)

type SessionStore struct {
	db *pgxpool.Pool
}

func NewSessionStore(db *pgxpool.Pool) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Get(r *http.Request, name string) (*http.Cookie, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	var userID string
	var expiresAt time.Time
	err = s.db.QueryRow(r.Context(),
		`SELECT user_id, expires_at FROM sessions WHERE token = $1 AND expires_at > NOW()`,
		c.Value,
	).Scan(&userID, &expiresAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *SessionStore) Save(w http.ResponseWriter, r *http.Request, name, value string, expiry time.Time) error {
	_, err := s.db.Exec(r.Context(),
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)
		 ON CONFLICT (token) DO UPDATE SET expires_at = $3`,
		value, value, expiry,
	)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Expires:  expiry,
		HttpOnly: true,
		Path:     "/",
	})
	return nil
}

func (s *SessionStore) Clear(w http.ResponseWriter, r *http.Request, name string) error {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
	})
	return nil
}

func (s *SessionStore) LoadUser(r *http.Request) *model.User {
	c, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	var u model.User
	err = s.db.QueryRow(r.Context(),
		`SELECT u.id, u.name, u.email, u.phone, u.password_hash, u.role, COALESCE(u.bank_account, ''), u.created_at
		 FROM users u JOIN sessions s ON s.user_id = u.id
		 WHERE s.token = $1 AND s.expires_at > NOW()`,
		c.Value,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.BankAccount, &u.CreatedAt)
	if err != nil {
		return nil
	}
	return &u
}

type AuthHandler struct {
	db *pgxpool.Pool
	ss *SessionStore
}

func NewAuthHandler(db *pgxpool.Pool, ss *SessionStore) *AuthHandler {
	return &AuthHandler{db: db, ss: ss}
}

func (h *AuthHandler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	templates.Register("").Render(r.Context(), w)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		templates.Register("Bad request").Render(r.Context(), w)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	phone := r.FormValue("phone")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")
	bankAccount := r.FormValue("bank_account")

	if name == "" || email == "" || phone == "" || password == "" {
		templates.Register("All required fields must be filled").Render(r.Context(), w)
		return
	}
	if password != confirmPassword {
		templates.Register("Passwords do not match").Render(r.Context(), w)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		templates.Register("Internal error").Render(r.Context(), w)
		return
	}

	var userID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO users (name, email, phone, password_hash, bank_account)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		name, email, phone, string(hash), nullIfEmpty(bankAccount),
	).Scan(&userID)
	if err != nil {
		templates.Register("Email already registered").Render(r.Context(), w)
		return
	}

	expiry := time.Now().Add(7 * 24 * time.Hour)
	if err := h.ss.Save(w, r, "session", userID, expiry); err != nil {
		templates.Register("Internal error").Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) LoginForm(w http.ResponseWriter, r *http.Request) {
	templates.Login("").Render(r.Context(), w)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		templates.Login("Bad request").Render(r.Context(), w)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	var u model.User
	err := h.db.QueryRow(r.Context(),
		`SELECT id, name, email, phone, password_hash, role, COALESCE(bank_account, ''), created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.BankAccount, &u.CreatedAt)
	if err != nil {
		templates.Login("Invalid email or password").Render(r.Context(), w)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		templates.Login("Invalid email or password").Render(r.Context(), w)
		return
	}

	expiry := time.Now().Add(7 * 24 * time.Hour)
	if err := h.ss.Save(w, r, "session", u.ID, expiry); err != nil {
		templates.Login("Internal error").Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.ss.Clear(w, r, "session")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
