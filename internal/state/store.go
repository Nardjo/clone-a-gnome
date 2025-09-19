package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Container conserve les infos sensibles nécessaires à la réutilisation d’un conteneur.
type Container struct {
	Name      string    `json:"name"`
	User      string    `json:"user"`
	Password  string    `json:"password"`
	Database  string    `json:"database"`
	Port      int       `json:"port"`
	Image     string    `json:"image"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store gère la persistance sur disque des conteneurs suivis.
type Store struct {
	path       string
	containers map[string]Container
}

// Open charge l’état courant depuis le disque (ou retourne un store vide).
func Open() (*Store, error) {
	path, err := defaultPath()
	if err != nil {
		return nil, err
	}

	st := &Store{path: path, containers: make(map[string]Container)}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return nil, fmt.Errorf("lecture de %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	data := struct {
		Containers []Container `json:"containers"`
	}{}
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("décodage du fichier d’état %s: %w", path, err)
	}

	for _, container := range data.Containers {
		st.containers[container.Name] = container
	}

	return st, nil
}

// Save écrit l’état courant sur disque.
func (s *Store) Save() error {
	if s == nil {
		return nil
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("création du dossier %s: %w", dir, err)
	}

	data := struct {
		Containers []Container `json:"containers"`
	}{Containers: make([]Container, 0, len(s.containers))}
	for _, container := range s.containers {
		data.Containers = append(data.Containers, container)
	}

	tmpPath := s.path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("ouverture de %s: %w", tmpPath, err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(&data); err != nil {
		_ = file.Close()
		return fmt.Errorf("écriture de %s: %w", tmpPath, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("fermeture de %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("renommage de %s vers %s: %w", tmpPath, s.path, err)
	}

	return nil
}

// Upsert enregistre ou met à jour un conteneur dans le store.
func (s *Store) Upsert(container Container) {
	if s == nil {
		return
	}

	existing, ok := s.containers[container.Name]
	if ok && !existing.CreatedAt.IsZero() {
		container.CreatedAt = existing.CreatedAt
	} else if container.CreatedAt.IsZero() {
		container.CreatedAt = time.Now()
	}
	container.UpdatedAt = time.Now()
	s.containers[container.Name] = container
}

// Get retourne un conteneur s’il existe.
func (s *Store) Get(name string) (Container, bool) {
	if s == nil {
		return Container{}, false
	}
	container, ok := s.containers[name]
	return container, ok
}

// Remove supprime un conteneur du suivi.
func (s *Store) Remove(name string) {
	if s == nil {
		return
	}
	delete(s.containers, name)
}

func defaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("impossible de déterminer le dossier de configuration utilisateur: %w", err)
		}
		configDir = filepath.Join(home, ".clone-a-gnome")
	} else {
		configDir = filepath.Join(configDir, "clone-a-gnome")
	}

	return filepath.Join(configDir, "state.json"), nil
}
