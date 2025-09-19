package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/jordanbastin/clone-a-gnome/internal/dockerx"
	"github.com/jordanbastin/clone-a-gnome/internal/logging"
	"github.com/jordanbastin/clone-a-gnome/internal/postgres"
	"github.com/jordanbastin/clone-a-gnome/internal/state"
)

func run(ctx context.Context, opts *Options) error {
	logger, err := logging.Configure(opts.LogLevel)
	if err != nil {
		return fmt.Errorf("configuration du logger: %w", err)
	}

	logger.Info("Validation de l’URL source", "url", opts.SourceURL)

	sourceInfo, err := postgres.ParseConnectionInfo(opts.SourceURL)
	if err != nil {
		logger.Error("URL source invalide", "err", err)
		return err
	}

	logger.Info("URL source validée", "hôte", sourceInfo.Host, "port", sourceInfo.Port, "base", sourceInfo.Database)

	if err := ensureBinaryAvailable("docker"); err != nil {
		logger.Error("Docker CLI introuvable", "err", err)
		return err
	}
	if err := ensureBinaryAvailable("pg_dump"); err != nil {
		logger.Error("pg_dump introuvable", "err", err)
		return err
	}

	store, err := state.Open()
	if err != nil {
		if opts.Reuse {
			return fmt.Errorf("impossible de charger l’état pour réutiliser un conteneur: %w", err)
		}
		logger.Warn("Impossible de charger l’état persistant, la réutilisation sera indisponible", "err", err)
		store = nil
	}

	containerUser := "local_user"
	databaseName := opts.LocalDBName
	targetPort := opts.LocalPort
	password := ""
	image := ""

	if opts.Reuse {
		if store == nil {
			return fmt.Errorf("l’option --reuse nécessite un état persistant valide")
		}
		saved, ok := store.Get(opts.ContainerName)
		if !ok {
			return fmt.Errorf("aucun conteneur nommé %s n’est enregistré : exécutez d’abord la commande sans --reuse", opts.ContainerName)
		}
		containerUser = saved.User
		password = saved.Password
		databaseName = saved.Database
		targetPort = saved.Port
		image = saved.Image
		if opts.LocalDBName != "" && opts.LocalDBName != saved.Database {
			logger.Warn("Le nom de base demandé diffère de celui enregistré; la valeur enregistrée sera utilisée", "demandé", opts.LocalDBName, "enregistré", saved.Database)
		}
		if opts.LocalPort != 0 && opts.LocalPort != saved.Port {
			logger.Warn("Le port demandé diffère de celui enregistré; le port enregistré sera conservé", "demandé", opts.LocalPort, "enregistré", saved.Port)
		}
	} else {
		var genErr error
		password, genErr = postgres.GeneratePassword(24)
		if genErr != nil {
			return fmt.Errorf("génération du mot de passe temporaire: %w", genErr)
		}
	}

	manager := dockerx.NewManager(logger)
	containerInfo, err := manager.Ensure(ctx, dockerx.Config{
		Name:     opts.ContainerName,
		User:     containerUser,
		Password: password,
		Database: databaseName,
		Port:     targetPort,
		Image:    image,
		Reuse:    opts.Reuse,
	})
	if err != nil {
		logger.Error("Échec de la préparation du conteneur", "err", err)
		return err
	}

	if store != nil {
		store.Upsert(state.Container{
			Name:     containerInfo.Name,
			User:     containerInfo.User,
			Password: containerInfo.Password,
			Database: containerInfo.Database,
			Port:     containerInfo.Port,
			Image:    containerInfo.Image,
		})
		if err := store.Save(); err != nil {
			logger.Warn("Impossible d’enregistrer l’état persistant", "err", err)
		}
	}

	target := postgres.Target{
		Container: containerInfo.Name,
		User:      containerInfo.User,
		Password:  containerInfo.Password,
		Database:  containerInfo.Database,
	}

	if err := postgres.StreamDump(ctx, logger, sourceInfo, target, opts.ExcludeTables); err != nil {
		return err
	}

	if opts.AnonymizeScript != "" {
		if err := postgres.RunSQLScript(ctx, logger, target, opts.AnonymizeScript); err != nil {
			return err
		}
	}

	logger.Info("Import terminé", "timestamp", time.Now().Format(time.RFC3339))
	fmt.Println()
	fmt.Println("URL locale prête :")
	fmt.Println(containerInfo.ConnectionURL())
	fmt.Println()
	stopCmd := fmt.Sprintf("docker stop %s", containerInfo.Name)
	logger.Warn("Pensez à arrêter le conteneur quand vous avez fini", "commande", stopCmd)
	fmt.Println(stopCmd)

	return nil
}

func ensureBinaryAvailable(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("binaire %s introuvable dans le PATH", name)
		}
		return fmt.Errorf("recherche du binaire %s: %w", name, err)
	}
	return nil
}
