package postgres

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"log/slog"

	"github.com/jordanbastin/clone-a-gnome/internal/logging"
)

// Target décrit la base locale vers laquelle on importe le dump.
type Target struct {
	Container string
	User      string
	Password  string
	Database  string
}

// StreamDump lance pg_dump puis restaure le flux directement dans psql dans le conteneur local.
func StreamDump(ctx context.Context, logger *slog.Logger, source *ConnectionInfo, target Target, excludeTables []string) error {
	dumpArgs := []string{"--no-owner", "--no-privileges", fmt.Sprintf("--dbname=%s", source.Raw)}
	for _, table := range excludeTables {
		dumpArgs = append(dumpArgs, fmt.Sprintf("--exclude-table=%s", table))
	}

	pgDump := exec.CommandContext(ctx, "pg_dump", dumpArgs...)
	pgDump.Stderr = logging.Writer(logger, slog.LevelWarn)

	dumpReader, err := pgDump.StdoutPipe()
	if err != nil {
		return fmt.Errorf("création du pipe pg_dump: %w", err)
	}

	psql := exec.CommandContext(ctx, "docker", buildPsqlArgs(target)...)
	psql.Stderr = logging.Writer(logger, slog.LevelWarn)
	psql.Stdout = logging.Writer(logger, slog.LevelInfo)
	psql.Stdin = dumpReader

	logger.Info("Lancement de pg_dump", "source", source.Masked, "exclusions", strings.Join(excludeTables, ","))
	if err := pgDump.Start(); err != nil {
		return fmt.Errorf("démarrage de pg_dump: %w", err)
	}

	logger.Info("Import en streaming vers le conteneur", "container", target.Container)
	if err := psql.Start(); err != nil {
		return fmt.Errorf("démarrage de psql (docker exec): %w", err)
	}

	if err := pgDump.Wait(); err != nil {
		return fmt.Errorf("pg_dump s’est terminé en erreur: %w", err)
	}

	if closer, ok := dumpReader.(io.Closer); ok {
		_ = closer.Close()
	}

	if err := psql.Wait(); err != nil {
		return fmt.Errorf("psql (docker exec) s’est terminé en erreur: %w", err)
	}

	return nil
}

// RunSQLScript exécute un fichier SQL dans la base locale.
func RunSQLScript(ctx context.Context, logger *slog.Logger, target Target, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("ouverture du script %s: %w", path, err)
	}
	defer file.Close()

	logger.Info("Exécution du script d’anonymisation", "script", path)
	cmd := exec.CommandContext(ctx, "docker", buildPsqlArgs(target)...)
	cmd.Stdin = file
	cmd.Stdout = logging.Writer(logger, slog.LevelInfo)
	cmd.Stderr = logging.Writer(logger, slog.LevelWarn)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("échec lors de l’exécution du script %s: %w", path, err)
	}
	return nil
}

func buildPsqlArgs(target Target) []string {
	return []string{"exec", "-i", "-e", fmt.Sprintf("PGPASSWORD=%s", target.Password), target.Container,
		"psql", "-U", target.User, "-d", target.Database}
}
