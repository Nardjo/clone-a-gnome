package dockerx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"log/slog"
)

const (
	defaultImage = "postgres:16-alpine"
	defaultHost  = "localhost"
)

// Config regroupe les paramètres nécessaires pour lancer le conteneur local.
type Config struct {
	Name     string
	Image    string
	User     string
	Password string
	Database string
	Port     int
	Reuse    bool
}

// Info représente l’état du conteneur PostgreSQL local.
type Info struct {
	ID       string
	Name     string
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Image    string
}

// ConnectionURL retourne l’URL de connexion complète vers la base locale.
func (i Info) ConnectionURL() string {
	user := urlQueryEscape(i.User)
	pass := urlQueryEscape(i.Password)
	db := urlQueryEscape(i.Database)
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, pass, i.Host, i.Port, db)
}

// Manager centralise les interactions avec la CLI Docker.
type Manager struct {
	logger *slog.Logger
}

// NewManager construit un manager Docker utilitaire.
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{logger: logger}
}

// Ensure provisionne ou réutilise un conteneur selon la configuration.
func (m *Manager) Ensure(ctx context.Context, cfg Config) (*Info, error) {
	if cfg.Image == "" {
		cfg.Image = defaultImage
	}

	if cfg.Name == "" {
		cfg.Name = "clone-a-gnome"
	}

	if cfg.User == "" {
		return nil, fmt.Errorf("le nom d’utilisateur local ne peut pas être vide")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("le mot de passe local ne peut pas être vide")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("le nom de base locale ne peut pas être vide")
	}

	if cfg.Port == 0 {
		port, err := findFreePort()
		if err != nil {
			return nil, fmt.Errorf("sélection du port local: %w", err)
		}
		cfg.Port = port
	}

	exists, containerID, err := m.containerExists(ctx, cfg.Name)
	if err != nil {
		return nil, err
	}

	if exists {
		if cfg.Reuse {
			m.logger.Info("Réutilisation d’un conteneur existant", "container", cfg.Name)
			info, err := m.inspect(ctx, containerID, cfg)
			if err != nil {
				return nil, err
			}
			return info, nil
		}
		return nil, fmt.Errorf("un conteneur nommé %s existe déjà; utilisez --reuse ou changez de nom", cfg.Name)
	}

	containerID, err = m.runContainer(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := m.waitForReady(ctx, cfg.Name, cfg.User, cfg.Database); err != nil {
		return nil, err
	}

	info := &Info{
		ID:       containerID,
		Name:     cfg.Name,
		Host:     defaultHost,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Database,
		Image:    cfg.Image,
	}

	return info, nil
}

func (m *Manager) runContainer(ctx context.Context, cfg Config) (string, error) {
	args := []string{"run", "-d", "--name", cfg.Name,
		"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", cfg.Password),
		"-e", fmt.Sprintf("POSTGRES_USER=%s", cfg.User),
		"-e", fmt.Sprintf("POSTGRES_DB=%s", cfg.Database),
		"-p", fmt.Sprintf("%d:5432", cfg.Port),
		cfg.Image,
	}

	m.logger.Info("Lancement du conteneur Docker", "image", cfg.Image, "container", cfg.Name, "port", cfg.Port)
	output, err := runCombinedOutput(ctx, "docker", args...)
	if err != nil {
		return "", fmt.Errorf("docker run: %w - %s", err, strings.TrimSpace(output))
	}

	id := strings.TrimSpace(output)
	if id == "" {
		return "", fmt.Errorf("docker run n’a pas retourné d’identifiant de conteneur")
	}

	return id, nil
}

func (m *Manager) containerExists(ctx context.Context, name string) (bool, string, error) {
	args := []string{"ps", "-a", "--filter", fmt.Sprintf("name=^/%s$", name), "--format", "{{.ID}}"}
	output, err := runCombinedOutput(ctx, "docker", args...)
	if err != nil {
		return false, "", fmt.Errorf("docker ps: %w - %s", err, strings.TrimSpace(output))
	}

	id := strings.TrimSpace(output)
	return id != "", id, nil
}

func (m *Manager) inspect(ctx context.Context, id string, cfg Config) (*Info, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect: %w - %s", err, strings.TrimSpace(string(output)))
	}

	var data []struct {
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
		State struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		} `json:"State"`
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("décodage de docker inspect: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("docker inspect n’a retourné aucune donnée pour %s", cfg.Name)
	}

	entry := data[0]

	if !entry.State.Running {
		m.logger.Info("Démarrage d’un conteneur existant", "container", cfg.Name)
		if err := m.startContainer(ctx, cfg.Name); err != nil {
			return nil, err
		}
		if err := m.waitForReady(ctx, cfg.Name, cfg.User, cfg.Database); err != nil {
			return nil, err
		}
		return m.inspect(ctx, id, cfg)
	}

	bindings := entry.NetworkSettings.Ports["5432/tcp"]
	port := cfg.Port
	if len(bindings) > 0 {
		port = atoi(bindings[0].HostPort)
	}
	image := cfg.Image
	if entry.Config.Image != "" {
		image = entry.Config.Image
	}

	return &Info{
		ID:       id,
		Name:     cfg.Name,
		Host:     defaultHost,
		Port:     port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Database,
		Image:    image,
	}, nil
}

func (m *Manager) startContainer(ctx context.Context, name string) error {
	output, err := runCombinedOutput(ctx, "docker", "start", name)
	if err != nil {
		return fmt.Errorf("docker start: %w - %s", err, strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) waitForReady(ctx context.Context, container, user, database string) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("le conteneur %s n’est pas prêt après 60s", container)
		}

		args := []string{"exec", container, "pg_isready", "-U", user, "-d", database}
		output, err := runCombinedOutput(ctx, "docker", args...)
		if err == nil {
			if strings.Contains(output, "accepting connections") {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

func atoi(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func runCombinedOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
