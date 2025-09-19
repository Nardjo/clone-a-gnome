package postgres

import (
	"fmt"
	"net/url"
	"strings"
)

// ConnectionInfo contient les détails décodés d’une URL PostgreSQL.
type ConnectionInfo struct {
	Raw        string
	Masked     string
	URL        *url.URL
	Host       string
	Port       string
	Database   string
	User       string
	Password   string
	Parameters map[string]string
}

// ParseConnectionInfo valide et normalise une URL de connexion PostgreSQL.
func ParseConnectionInfo(raw string) (*ConnectionInfo, error) {
	if raw == "" {
		return nil, fmt.Errorf("URL vide")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("URL invalide: %w", err)
	}

	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return nil, fmt.Errorf("schéma %q non supporté (attendu postgres ou postgresql)", parsed.Scheme)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("aucun hôte n’est défini dans l’URL")
	}

	database := strings.TrimPrefix(parsed.Path, "/")
	if database == "" {
		return nil, fmt.Errorf("merci de préciser le nom de la base dans l’URL")
	}

	user := ""
	pass := ""
	if parsed.User != nil {
		user = parsed.User.Username()
		pass, _ = parsed.User.Password()
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}

	params := make(map[string]string)
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			params[key] = values[len(values)-1]
		}
	}

	masked := maskPassword(parsed)

	return &ConnectionInfo{
		Raw:        raw,
		Masked:     masked,
		URL:        parsed,
		Host:       host,
		Port:       port,
		Database:   database,
		User:       user,
		Password:   pass,
		Parameters: params,
	}, nil
}

func maskPassword(u *url.URL) string {
	if u == nil || u.User == nil {
		return u.String()
	}

	username := u.User.Username()
	if username == "" {
		return u.Redacted()
	}

	clone := *u
	clone.User = url.User(username)
	return clone.String()
}
