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
	config *Config
	wg     *WireGuardManager
	pf     *PortForwardManager
	store  *sessions.CookieStore
	tmpl   *template.Template
}

func NewServer(config *Config, wg *WireGuardManager, pf *PortForwardManager) *Server {
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
		pf:     pf,
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
        .btn-portforward { background: #6f42c1; color: white; }
        .btn-portforward:hover { background: #5a32a3; }
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
                    <a href="{{$.BasePath}}/clients/{{.ID}}/portforwards" class="btn btn-portforward">🔌 Ports</a>
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

// Port forwarding handlers

func (s *Server) handlePortForwards(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	client, err := s.wg.GetClient(clientID)
	if err != nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	mappings := s.pf.GetClientMappings(clientID)
	externalIP, _ := s.pf.GetExternalIP()

	s.renderPortForwards(w, client, mappings, externalIP, "")
}

func (s *Server) handleAddPortForward(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	clientID := vars["id"]

	client, err := s.wg.GetClient(clientID)
	if err != nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Parse form
	externalPort := r.FormValue("external_port")
	internalPort := r.FormValue("internal_port")
	protocol := r.FormValue("protocol")
	description := r.FormValue("description")

	// Validate and convert ports
	var extPort, intPort uint16
	if _, err := fmt.Sscanf(externalPort, "%d", &extPort); err != nil || extPort == 0 {
		mappings := s.pf.GetClientMappings(clientID)
		externalIP, _ := s.pf.GetExternalIP()
		s.renderPortForwards(w, client, mappings, externalIP, "Invalid external port")
		return
	}
	if _, err := fmt.Sscanf(internalPort, "%d", &intPort); err != nil || intPort == 0 {
		mappings := s.pf.GetClientMappings(clientID)
		externalIP, _ := s.pf.GetExternalIP()
		s.renderPortForwards(w, client, mappings, externalIP, "Invalid internal port")
		return
	}

	// Get client's internal IP (IPv4)
	internalIP := client.AddressV4
	if idx := len(internalIP) - 3; idx > 0 {
		internalIP = internalIP[:idx] // Remove /32
	}

	// Add port forward
	err = s.pf.AddMapping(clientID, client.Name, internalIP, extPort, intPort, protocol, description)
	if err != nil {
		mappings := s.pf.GetClientMappings(clientID)
		externalIP, _ := s.pf.GetExternalIP()
		s.renderPortForwards(w, client, mappings, externalIP, err.Error())
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/clients/%s/portforwards", s.config.BasePath, clientID), http.StatusSeeOther)
}

func (s *Server) handleDeletePortForward(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	clientID := vars["id"]
	externalPort := vars["port"]
	protocol := vars["protocol"]

	var port uint16
	if _, err := fmt.Sscanf(externalPort, "%d", &port); err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	if err := s.pf.RemoveMapping(clientID, port, protocol); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/clients/%s/portforwards", s.config.BasePath, clientID), http.StatusSeeOther)
}

func (s *Server) handleAPIPortForwards(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["id"]

	mappings := s.pf.GetClientMappings(clientID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mappings)
}

func (s *Server) handleAPIAllPortForwards(w http.ResponseWriter, r *http.Request) {
	mappings := s.pf.GetAllMappings()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mappings)
}

func (s *Server) renderPortForwards(w http.ResponseWriter, client *WireGuardClient, mappings []*PortMapping, externalIP, errorMsg string) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Port Forwards - {{.Client.Name}}</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .back { padding: 8px 16px; background: #6c757d; color: white; text-decoration: none; border-radius: 4px; }
        .back:hover { background: #5a6268; }
        .info-box { background: #e7f3ff; padding: 15px; border-radius: 8px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .info-box strong { color: #007bff; }
        .add-form { background: #f8f9fa; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .add-form input, .add-form select { padding: 10px; margin-right: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .add-form button { padding: 10px 20px; background: #28a745; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .add-form button:hover { background: #218838; }
        .error { background: #f8d7da; color: #721c24; padding: 12px; border-radius: 4px; margin-bottom: 20px; border-left: 4px solid #dc3545; }
        .disabled { background: #fff3cd; color: #856404; padding: 15px; border-radius: 8px; margin-bottom: 20px; border-left: 4px solid #ffc107; }
        table { width: 100%; border-collapse: collapse; background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #007bff; color: white; }
        tr:hover { background: #f8f9fa; }
        .actions { display: flex; gap: 10px; }
        .btn { padding: 6px 12px; text-decoration: none; border-radius: 4px; border: none; cursor: pointer; font-size: 14px; }
        .btn-delete { background: #dc3545; color: white; }
        .btn-delete:hover { background: #c82333; }
        .empty { text-align: center; padding: 40px; color: #666; }
        .code { font-family: monospace; font-size: 12px; color: #666; }
        .form-row { margin-bottom: 10px; }
        .form-row label { display: inline-block; width: 120px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔌 Port Forwards - {{.Client.Name}}</h1>
        <a href="{{.BasePath}}/" class="back">← Back to Clients</a>
    </div>

    {{if .ExternalIP}}
    <div class="info-box">
        <strong>External IP:</strong> {{.ExternalIP}}<br>
        <strong>Client Internal IP:</strong> {{.Client.AddressV4}}
    </div>
    {{end}}

    {{if .Error}}
    <div class="error">{{.Error}}</div>
    {{end}}

    {{if .Enabled}}
    <div class="add-form">
        <h2>Add Port Forward</h2>
        <form method="POST" action="{{.BasePath}}/clients/{{.Client.ID}}/portforwards/add">
            <div class="form-row">
                <label>External Port:</label>
                <input type="number" name="external_port" min="1024" max="65535" required>
            </div>
            <div class="form-row">
                <label>Internal Port:</label>
                <input type="number" name="internal_port" min="1" max="65535" required>
            </div>
            <div class="form-row">
                <label>Protocol:</label>
                <select name="protocol" required>
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                </select>
            </div>
            <div class="form-row">
                <label>Description:</label>
                <input type="text" name="description" placeholder="e.g., Web Server" required>
            </div>
            <button type="submit">➕ Add Port Forward</button>
        </form>
    </div>

    {{if .Mappings}}
    <table>
        <thead>
            <tr>
                <th>External Port</th>
                <th>Internal Port</th>
                <th>Protocol</th>
                <th>Description</th>
                <th>Created</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
            {{range .Mappings}}
            <tr>
                <td><strong>{{.ExternalPort}}</strong></td>
                <td class="code">{{.InternalIP}}:{{.InternalPort}}</td>
                <td>{{.Protocol | upper}}</td>
                <td>{{.Description}}</td>
                <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                <td class="actions">
                    <form method="POST" action="{{$.BasePath}}/clients/{{$.Client.ID}}/portforwards/{{.ExternalPort}}/{{.Protocol}}/delete" style="display: inline;">
                        <button type="submit" class="btn btn-delete" onclick="return confirm('Delete port forward {{.ExternalPort}}?')">🗑️ Delete</button>
                    </form>
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{else}}
    <div class="empty">
        <p>No port forwards configured. Add one above!</p>
    </div>
    {{end}}
    {{else}}
    <div class="disabled">
        <strong>⚠️ Port forwarding is disabled</strong><br>
        Enable it in the server configuration to use this feature.
    </div>
    {{end}}
</body>
</html>`

	t := template.Must(template.New("portforwards").Funcs(template.FuncMap{
		"upper": func(s string) string {
			return fmt.Sprintf("%s", s)
		},
	}).Parse(tmpl))

	t.Execute(w, map[string]interface{}{
		"Client":     client,
		"Mappings":   mappings,
		"ExternalIP": externalIP,
		"Error":      errorMsg,
		"Enabled":    s.pf.IsEnabled(),
		"BasePath":   s.config.BasePath,
	})
}
