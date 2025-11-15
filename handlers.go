package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

type Server struct {
	config  *Config
	wg      *WireGuardManager
	store   *sessions.CookieStore
	tmpl    *template.Template
}

func NewServer(config *Config, wg *WireGuardManager) *Server {
	store := sessions.NewCookieStore([]byte(config.SessionSecret))
	store.Options = &sessions.Options{
		Path:     config.BasePath + "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	return &Server{
		config: config,
		wg:     wg,
		store:  store,
	}
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	session, _ := s.store.Get(r, "session")
	auth, ok := session.Values["authenticated"].(bool)
	return ok && auth
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			http.Redirect(w, r, s.config.BasePath+"/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s.renderLogin(w, "")
		return
	}

	if r.Method == "POST" {
		password := r.FormValue("password")
		
		if password == s.config.AdminPassword {
			session, _ := s.store.Get(r, "session")
			session.Values["authenticated"] = true
			session.Save(r, w)
			http.Redirect(w, r, s.config.BasePath+"/", http.StatusSeeOther)
			return
		}

		s.renderLogin(w, "Invalid password")
		return
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "session")
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, s.config.BasePath+"/login", http.StatusSeeOther)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	clients := s.wg.GetClients()
	s.renderIndex(w, clients)
}

func (s *Server) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	client, err := s.wg.CreateClient(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.config.BasePath+"/", http.StatusSeeOther)
	_ = client
}

func (s *Server) handleDeleteClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.wg.DeleteClient(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, s.config.BasePath+"/", http.StatusSeeOther)
}

func (s *Server) handleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	client, err := s.wg.GetClient(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	config := s.wg.GenerateClientConfig(client)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.conf", client.Name))
	w.Write([]byte(config))
}

func (s *Server) handleAPIClients(w http.ResponseWriter, r *http.Request) {
	clients := s.wg.GetClients()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func (s *Server) renderLogin(w http.ResponseWriter, errorMsg string) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>WireGuard Easy - Login</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
        input { width: 100%; padding: 10px; margin: 10px 0; box-sizing: border-box; }
        button { width: 100%; padding: 10px; background: #007bff; color: white; border: none; cursor: pointer; }
        button:hover { background: #0056b3; }
        .error { color: red; margin: 10px 0; }
        h1 { text-align: center; }
    </style>
</head>
<body>
    <h1>🔒 WireGuard Easy</h1>
    {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
    <form method="POST" action="{{.BasePath}}/login">
        <input type="password" name="password" placeholder="Admin Password" required autofocus>
        <button type="submit">Login</button>
    </form>
</body>
</html>`

	t := template.Must(template.New("login").Parse(tmpl))
	t.Execute(w, map[string]interface{}{
		"Error":    errorMsg,
		"BasePath": s.config.BasePath,
	})
}

func (s *Server) renderIndex(w http.ResponseWriter, clients []*WireGuardClient) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>WireGuard Easy</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .logout { padding: 8px 16px; background: #dc3545; color: white; text-decoration: none; border-radius: 4px; }
        .logout:hover { background: #c82333; }
        .add-form { background: #f8f9fa; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .add-form input { padding: 10px; margin-right: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .add-form button { padding: 10px 20px; background: #28a745; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .add-form button:hover { background: #218838; }
        table { width: 100%; border-collapse: collapse; background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #007bff; color: white; }
        tr:hover { background: #f8f9fa; }
        .actions { display: flex; gap: 10px; }
        .btn { padding: 6px 12px; text-decoration: none; border-radius: 4px; border: none; cursor: pointer; font-size: 14px; }
        .btn-download { background: #17a2b8; color: white; }
        .btn-download:hover { background: #138496; }
        .btn-delete { background: #dc3545; color: white; }
        .btn-delete:hover { background: #c82333; }
        .empty { text-align: center; padding: 40px; color: #666; }
        .code { font-family: monospace; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔐 WireGuard Easy</h1>
        <a href="{{.BasePath}}/logout" class="logout">Logout</a>
    </div>

    <div class="add-form">
        <h2>Add New Client</h2>
        <form method="POST" action="{{.BasePath}}/clients/create">
            <input type="text" name="name" placeholder="Client Name" required>
            <button type="submit">➕ Add Client</button>
        </form>
    </div>

    {{if .Clients}}
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>IPv4 Address</th>
                <th>IPv6 Address</th>
                <th>Public Key</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
            {{range .Clients}}
            <tr>
                <td><strong>{{.Name}}</strong></td>
                <td class="code">{{.AddressV4}}</td>
                <td class="code">{{.AddressV6}}</td>
                <td class="code">{{slice .PublicKey 0 20}}...</td>
                <td class="actions">
                    <a href="{{$.BasePath}}/clients/{{.ID}}/config" class="btn btn-download">📥 Download</a>
                    <form method="POST" action="{{$.BasePath}}/clients/{{.ID}}/delete" style="display: inline;">
                        <button type="submit" class="btn btn-delete" onclick="return confirm('Delete {{.Name}}?')">🗑️ Delete</button>
                    </form>
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <div class="empty">
        <p>No clients yet. Add your first client above!</p>
    </div>
    {{end}}
</body>
</html>`

	t := template.Must(template.New("index").Funcs(template.FuncMap{
		"slice": func(s string, start, end int) string {
			if len(s) < end {
				return s
			}
			return s[start:end]
		},
	}).Parse(tmpl))

	t.Execute(w, map[string]interface{}{
		"Clients":  clients,
		"BasePath": s.config.BasePath,
	})
}


