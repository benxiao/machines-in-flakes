package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ---- Auth ----

type ctxKey string

const ctxUserID ctxKey = "user_id"
const ctxIsAdmin ctxKey = "is_admin"

func isAdmin(r *http.Request) bool {
	v, _ := r.Context().Value(ctxIsAdmin).(bool)
	return v
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *App) withAuth(next http.Handler) http.Handler {
	exempt := map[string]bool{
		"/login": true, "/logout": true, "/favicon.svg": true,
		"/auth/google/login": true, "/auth/google/callback": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if exempt[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("fb_session")
		if err != nil {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		var userID int64
		var admin bool
		err = a.db.QueryRow(r.Context(), `
			SELECT s.user_id, u.is_admin FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token = $1 AND s.expires_at > now()
		`, cookie.Value).Scan(&userID, &admin)
		if err != nil {
			http.SetCookie(w, &http.Cookie{Name: "fb_session", Value: "", Path: "/", MaxAge: -1})
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxIsAdmin, admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) bootstrapAdmin(ctx context.Context, username, password string) {
	if username == "" || password == "" {
		return
	}
	var count int
	if err := a.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil || count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bootstrap admin: %v", err)
		return
	}
	if _, err = a.db.Exec(ctx, `INSERT INTO users (username, password_hash, is_admin) VALUES ($1, $2, TRUE)`, username, string(hash)); err != nil {
		log.Printf("bootstrap admin: %v", err)
	} else {
		log.Printf("created admin user %q", username)
	}
}

func (a *App) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	render(w, "login", LoginPage{Next: r.URL.Query().Get("next")})
}

func (a *App) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	next := r.FormValue("next")

	var userID int64
	var hash string
	err := a.db.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE username = $1`, username,
	).Scan(&userID, &hash)
	const errMsg = "Invalid username or password."
	if err != nil {
		render(w, "login", LoginPage{Error: errMsg, Next: next})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		render(w, "login", LoginPage{Error: errMsg, Next: next})
		return
	}
	token := randToken()
	if _, err = a.db.Exec(r.Context(), `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, now() + interval '30 days')
	`, token, userID); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "fb_session",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	// Require a local path: "//host" and "/\host" are protocol-relative
	// URLs to browsers, i.e. open redirects.
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, `/\`) {
		next = "/browse"
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("fb_session"); err == nil {
		a.db.Exec(r.Context(), `DELETE FROM sessions WHERE token = $1`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "fb_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ---- Google sign-in ----
//
// Deliberately does NOT auto-provision accounts: a Google account only
// works for login once an admin has linked it to an existing user via
// handleUserSetGoogleEmail. This mirrors the existing admin-managed-users
// model instead of adding open signup to a private media server.

func googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("FB_GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("FB_GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("FB_GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"openid", "email"},
		Endpoint:     google.Endpoint,
	}
}

func (a *App) handleGoogleLoginStart(w http.ResponseWriter, r *http.Request) {
	cfg := googleOAuthConfig()
	if cfg.ClientID == "" {
		http.Error(w, "Google sign-in is not configured", http.StatusServiceUnavailable)
		return
	}
	state := randToken()
	http.SetCookie(w, &http.Cookie{Name: "fb_oauth_state", Value: state, Path: "/", MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	next := r.URL.Query().Get("next")
	http.SetCookie(w, &http.Cookie{Name: "fb_oauth_next", Value: next, Path: "/", MaxAge: 600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusFound)
}

func (a *App) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	const failMsg = "Google sign-in failed. Please try again."

	stateCookie, stateErr := r.Cookie("fb_oauth_state")
	http.SetCookie(w, &http.Cookie{Name: "fb_oauth_state", Value: "", Path: "/", MaxAge: -1})
	next := "/browse"
	if nextCookie, err := r.Cookie("fb_oauth_next"); err == nil && strings.HasPrefix(nextCookie.Value, "/") &&
		!strings.HasPrefix(nextCookie.Value, "//") && !strings.HasPrefix(nextCookie.Value, `/\`) {
		next = nextCookie.Value
	}
	http.SetCookie(w, &http.Cookie{Name: "fb_oauth_next", Value: "", Path: "/", MaxAge: -1})

	if stateErr != nil || r.URL.Query().Get("state") != stateCookie.Value {
		render(w, "login", LoginPage{Error: "Google sign-in failed (session expired). Please try again."})
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		render(w, "login", LoginPage{Error: "Google sign-in was cancelled."})
		return
	}

	cfg := googleOAuthConfig()
	token, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("google oauth exchange: %v", err)
		render(w, "login", LoginPage{Error: failMsg})
		return
	}
	resp, err := cfg.Client(r.Context(), token).Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		log.Printf("google oauth userinfo: %v", err)
		render(w, "login", LoginPage{Error: failMsg})
		return
	}
	defer resp.Body.Close()
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Email == "" {
		log.Printf("google oauth userinfo decode: %v", err)
		render(w, "login", LoginPage{Error: failMsg})
		return
	}
	if !info.EmailVerified {
		render(w, "login", LoginPage{Error: "Your Google account's email address is not verified."})
		return
	}

	var userID int64
	err = a.db.QueryRow(r.Context(), `SELECT id FROM users WHERE google_email = $1`, strings.ToLower(info.Email)).Scan(&userID)
	if err != nil {
		render(w, "login", LoginPage{Error: fmt.Sprintf("The Google account %s isn't linked to a filebrowser user. Ask an admin to link it.", info.Email)})
		return
	}

	sessToken := randToken()
	if _, err = a.db.Exec(r.Context(), `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, now() + interval '30 days')
	`, sessToken, userID); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "fb_session",
		Value:    sessToken,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, next, http.StatusFound)
}

// ---- User management ----

func (a *App) handleUserList(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id, username, created_at FROM users ORDER BY created_at`)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var users []UserRow
	for rows.Next() {
		var u UserRow
		var t time.Time
		if rows.Scan(&u.ID, &u.Username, &t) == nil {
			u.CreatedAt = t.Format("2006-01-02 15:04")
			users = append(users, u)
		}
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	render(w, "users", UsersPage{ActiveTab: "users", IsAdmin: isAdmin(r), Users: users, CurrentUID: currentUID})
}

func (a *App) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	targetID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var username, googleEmail string
	if err := a.db.QueryRow(r.Context(),
		`SELECT username, COALESCE(google_email, '') FROM users WHERE id = $1`, targetID,
	).Scan(&username, &googleEmail); err != nil {
		http.NotFound(w, r)
		return
	}
	// Load all admin paths with granted flag for this user.
	rows, err := a.db.Query(r.Context(), `
		SELECT ip.id, ip.path, (pg.path_id IS NOT NULL) AS granted
		FROM indexed_paths ip
		LEFT JOIN path_grants pg ON pg.path_id = ip.id AND pg.user_id = $2
		WHERE ip.user_id = $1
		ORDER BY ip.path`, uid(r), targetID)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	defer rows.Close()
	var allPaths []AdminPathRow
	for rows.Next() {
		var p AdminPathRow
		if rows.Scan(&p.ID, &p.Path, &p.Granted) == nil {
			allPaths = append(allPaths, p)
		}
	}
	render(w, "user_detail", UserDetailPage{
		ActiveTab:   "users",
		IsAdmin:     true,
		ID:          targetID,
		Username:    username,
		GoogleEmail: googleEmail,
		AllPaths:    allPaths,
	})
}

// handleUserSetGoogleEmail links or unlinks a Google account for sign-in
// (see handleGoogleCallback): an empty value clears the link. Google
// sign-in only ever matches an EXISTING user by this field -- it never
// creates accounts -- so this is the only way a Google account becomes
// usable for login.
func (a *App) handleUserSetGoogleEmail(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	targetID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	email := strings.TrimSpace(strings.ToLower(r.FormValue("google_email")))
	var err error
	if email == "" {
		_, err = a.db.Exec(r.Context(), `UPDATE users SET google_email = NULL WHERE id = $1`, targetID)
	} else {
		_, err = a.db.Exec(r.Context(), `UPDATE users SET google_email = $1 WHERE id = $2`, email, targetID)
	}
	if err != nil {
		msg := "Failed to update Google account link."
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			msg = fmt.Sprintf("%s is already linked to another user.", email)
		}
		var username, googleEmail string
		a.db.QueryRow(r.Context(), `SELECT username, COALESCE(google_email, '') FROM users WHERE id = $1`, targetID).Scan(&username, &googleEmail)
		render(w, "user_detail", UserDetailPage{ActiveTab: "users", IsAdmin: true, ID: targetID, Username: username, GoogleEmail: googleEmail, Error: msg})
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/users/%d", targetID), http.StatusFound)
}

func (a *App) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		a.renderUsersWithError(w, r, "Username and password are required.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		httpErr(w, err, 500)
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO users (username, password_hash) VALUES ($1, $2)`, username, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			a.renderUsersWithError(w, r, fmt.Sprintf("Username %q already exists.", username))
			return
		}
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (a *App) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	if id == currentUID {
		a.renderUsersWithError(w, r, "You cannot delete your own account.")
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM users WHERE id = $1`, id); err != nil {
		httpErr(w, err, 500)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (a *App) renderUsersWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	rows, _ := a.db.Query(r.Context(), `SELECT id, username, created_at FROM users ORDER BY created_at`)
	var users []UserRow
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var u UserRow
			var t time.Time
			if rows.Scan(&u.ID, &u.Username, &t) == nil {
				u.CreatedAt = t.Format("2006-01-02 15:04")
				users = append(users, u)
			}
		}
	}
	currentUID, _ := r.Context().Value(ctxUserID).(int64)
	render(w, "users", UsersPage{ActiveTab: "users", IsAdmin: isAdmin(r), Users: users, CurrentUID: currentUID, Error: errMsg})
}

func (a *App) ownsPath(r *http.Request, pathID int64) bool {
	var count int
	a.db.QueryRow(r.Context(), `SELECT COUNT(*) FROM indexed_paths WHERE id=$1 AND user_id=$2`, pathID, uid(r)).Scan(&count)
	return count > 0
}

func (a *App) handlePathGrant(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pathID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	grantUID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || grantUID <= 0 {
		http.Redirect(w, r, "/users", http.StatusFound)
		return
	}
	if !a.ownsPath(r, pathID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	a.db.Exec(r.Context(), `INSERT INTO path_grants (path_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, pathID, grantUID)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, grantUID)
	}()
	http.Redirect(w, r, fmt.Sprintf("/users/%d", grantUID), http.StatusFound)
}

func (a *App) handlePathRevoke(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pathID, ok := parseID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	revokeUID, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil || revokeUID <= 0 {
		http.NotFound(w, r)
		return
	}
	if !a.ownsPath(r, pathID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	a.db.Exec(r.Context(), `DELETE FROM path_grants WHERE path_id=$1 AND user_id=$2`, pathID, revokeUID)
	go func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		a.reindexUser(ctx2, revokeUID)
	}()
	http.Redirect(w, r, fmt.Sprintf("/users/%d", revokeUID), http.StatusFound)
}
