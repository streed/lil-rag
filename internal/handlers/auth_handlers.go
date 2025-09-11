package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// LoginRequest represents the login request payload
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Username string `json:"username,omitempty"`
}

// Login handles user authentication
func (h *Handler) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.auth == nil {
			h.writeError(w, http.StatusServiceUnavailable, "authentication not configured", "")
			return
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid JSON", err.Error())
			return
		}

		if req.Username == "" || req.Password == "" {
			h.writeError(w, http.StatusBadRequest, "username and password required", "")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Authenticate user
		user, err := h.auth.AuthenticateUser(ctx, req.Username, req.Password)
		if err != nil {
			h.writeError(w, http.StatusUnauthorized, "invalid credentials", "")
			return
		}

		// Create session
		session, err := h.auth.CreateSession(ctx, user.ID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to create session", err.Error())
			return
		}

		// Set cookie
		cookie := &http.Cookie{
			Name:     "session_token",
			Value:    session.Token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
			SameSite: http.SameSiteLaxMode,
			Expires:  session.ExpiresAt,
		}
		http.SetCookie(w, cookie)

		w.Header().Set("Content-Type", "application/json")
		response := LoginResponse{
			Success:  true,
			Message:  "Login successful",
			Username: user.Username,
		}
		json.NewEncoder(w).Encode(response)
	}
}

// Logout handles user logout
func (h *Handler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		if h.auth == nil {
			h.writeError(w, http.StatusServiceUnavailable, "authentication not configured", "")
			return
		}

		// Get session token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			// No cookie, already logged out
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Logged out successfully",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Delete session from database
		h.auth.DeleteSession(ctx, cookie.Value)

		// Clear cookie
		clearCookie := &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Unix(0, 0),
		}
		http.SetCookie(w, clearCookie)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Logged out successfully",
		})
	}
}

// AuthStatus checks if the user is authenticated
func (h *Handler) AuthStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// If security is disabled, return no auth required
		if !h.secure {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authenticated": true,
				"auth_required": false,
			})
			return
		}

		if h.auth == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authenticated": false,
				"auth_required": false,
			})
			return
		}

		// Check if authentication is required
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		hasUsers, err := h.auth.HasUsers(ctx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to check users", err.Error())
			return
		}

		if !hasUsers {
			// No users exist, no authentication required
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authenticated": true,
				"auth_required": false,
			})
			return
		}

		// Get session token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authenticated": false,
				"auth_required": true,
			})
			return
		}

		// Validate session
		session, err := h.auth.ValidateSession(ctx, cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"authenticated": false,
				"auth_required": true,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"auth_required": true,
			"username":      session.Username,
		})
	}
}

// LoginPage serves the login page
func (h *Handler) LoginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
			return
		}

		// Simple login page HTML
		loginHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Lil RAG</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        
        .login-container {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 25px rgba(0,0,0,0.1);
            width: 100%;
            max-width: 400px;
        }
        
        .login-header {
            text-align: center;
            margin-bottom: 2rem;
        }
        
        .login-header h1 {
            color: #333;
            margin-bottom: 0.5rem;
        }
        
        .login-header p {
            color: #666;
        }
        
        .form-group {
            margin-bottom: 1rem;
        }
        
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            color: #333;
            font-weight: 500;
        }
        
        .form-group input {
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #ddd;
            border-radius: 5px;
            font-size: 1rem;
        }
        
        .form-group input:focus {
            outline: none;
            border-color: #667eea;
        }
        
        .login-button {
            width: 100%;
            background: #667eea;
            color: white;
            padding: 0.75rem;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            cursor: pointer;
            transition: background 0.2s;
        }
        
        .login-button:hover {
            background: #5a67d8;
        }
        
        .login-button:disabled {
            background: #ccc;
            cursor: not-allowed;
        }
        
        .error {
            color: #e53e3e;
            margin-top: 1rem;
            text-align: center;
        }
        
        .success {
            color: #38a169;
            margin-top: 1rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-header">
            <h1>Lil RAG</h1>
            <p>Please sign in to continue</p>
        </div>
        
        <form id="loginForm">
            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required>
            </div>
            
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            
            <button type="submit" class="login-button" id="loginButton">Sign In</button>
            
            <div id="message"></div>
        </form>
    </div>

    <script>
        document.getElementById('loginForm').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const username = document.getElementById('username').value;
            const password = document.getElementById('password').value;
            const button = document.getElementById('loginButton');
            const message = document.getElementById('message');
            
            button.disabled = true;
            button.textContent = 'Signing in...';
            message.textContent = '';
            
            try {
                const response = await fetch('/api/auth/login', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ username, password }),
                });
                
                const data = await response.json();
                
                if (data.success) {
                    message.className = 'success';
                    message.textContent = 'Login successful! Redirecting...';
                    setTimeout(() => {
                        window.location.href = '/';
                    }, 1000);
                } else {
                    message.className = 'error';
                    message.textContent = data.message || 'Login failed';
                    button.disabled = false;
                    button.textContent = 'Sign In';
                }
            } catch (error) {
                message.className = 'error';
                message.textContent = 'Network error. Please try again.';
                button.disabled = false;
                button.textContent = 'Sign In';
            }
        });
    </script>
</body>
</html>`

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(loginHTML))
	}
}

// AuthMiddleware checks authentication for protected routes
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth check if security is disabled
		if !h.secure {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth check if no auth system configured
		if h.auth == nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Check if any users exist
		hasUsers, err := h.auth.HasUsers(ctx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "authentication error", err.Error())
			return
		}

		// If no users exist, allow access without authentication
		if !hasUsers {
			next.ServeHTTP(w, r)
			return
		}

		// Get session token from cookie
		cookie, err := r.Cookie("session_token")
		if err != nil {
			// No session cookie, redirect to login
			if r.Header.Get("Accept") == "application/json" || r.Header.Get("Content-Type") == "application/json" {
				h.writeError(w, http.StatusUnauthorized, "authentication required", "")
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}

		// Validate session
		session, err := h.auth.ValidateSession(ctx, cookie.Value)
		if err != nil {
			// Invalid session, redirect to login
			if r.Header.Get("Accept") == "application/json" || r.Header.Get("Content-Type") == "application/json" {
				h.writeError(w, http.StatusUnauthorized, "invalid session", "")
			} else {
				http.Redirect(w, r, "/login", http.StatusFound)
			}
			return
		}

		// Add user info to request context
		ctx = context.WithValue(r.Context(), "user", session)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
