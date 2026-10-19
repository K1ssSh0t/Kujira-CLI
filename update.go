package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const (
	updateRepoSlug        = "K1ssSh0t/Kujira-CLI"
	updateChecksumsFile   = "checksums.txt"
	updateCheckInterval   = 24 * time.Hour
	updateCheckTimeout    = 3 * time.Second
	updateLookupTimeout   = 15 * time.Second
	updateDownloadTimeout = 5 * time.Minute
)

var (
	errNoPublishedRelease = errors.New("no hay releases publicados")
	// noUpdateCheck se activa con --no-update-check.
	noUpdateCheck bool
)

type updateCheckCache struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

type updateOptions struct {
	CheckOnly  bool
	Yes        bool
	Force      bool
	Prerelease bool
}

func getUpdateCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kujira_cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "update_check.json"), nil
}

func readUpdateCache() (updateCheckCache, bool) {
	var cache updateCheckCache
	path, err := getUpdateCachePath()
	if err != nil {
		return cache, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache, false
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return cache, false
	}
	return cache, true
}

func writeUpdateCache(latestVersion string) error {
	path, err := getUpdateCachePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(updateCheckCache{
		CheckedAt:     time.Now(),
		LatestVersion: latestVersion,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func newUpdateUpdater(prerelease bool) (*selfupdate.Updater, error) {
	return selfupdate.NewUpdater(selfupdate.Config{
		Validator:  &selfupdate.ChecksumValidator{UniqueFilename: updateChecksumsFile},
		Prerelease: prerelease,
	})
}

func detectLatestRelease(ctx context.Context, prerelease bool) (*selfupdate.Release, error) {
	updater, err := newUpdateUpdater(prerelease)
	if err != nil {
		return nil, err
	}
	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(updateRepoSlug))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errNoPublishedRelease
	}
	return latest, nil
}

func parseCurrentVersion() (*semver.Version, error) {
	v := strings.TrimSpace(version)
	if v == "" || v == "dev" {
		return nil, fmt.Errorf("versión de desarrollo")
	}
	return semver.NewVersion(v)
}

func formatReleaseVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "desconocida"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// checkUpdatesOnStartup busca actualizaciones como máximo una vez al día y
// guarda el aviso para mostrarlo en el menú principal. Los errores se ignoran
// para no molestar al usuario si no hay conexión.
func (c *CLI) checkUpdatesOnStartup() {
	if noUpdateCheck || os.Getenv("KUJIRA_NO_UPDATE_CHECK") == "1" {
		return
	}

	current, err := parseCurrentVersion()
	if err != nil {
		return
	}

	if cache, ok := readUpdateCache(); ok && time.Since(cache.CheckedAt) < updateCheckInterval {
		c.setUpdateNotice(cache.LatestVersion, current)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	latest, err := detectLatestRelease(ctx, false)
	if err != nil {
		// Guardar el intento evita repetir la espera en cada inicio si no hay
		// red o todavía no existe un release.
		_ = writeUpdateCache("")
		return
	}

	_ = writeUpdateCache(latest.Version())
	c.setUpdateNotice(latest.Version(), current)
}

func (c *CLI) setUpdateNotice(latestVersion string, current *semver.Version) {
	if latestVersion == "" {
		return
	}
	latest, err := semver.NewVersion(latestVersion)
	if err != nil {
		return
	}
	if latest.GreaterThan(current) {
		c.updateNotice = fmt.Sprintf("Nueva versión %s disponible. Ejecuta `kujira-cli update`.", formatReleaseVersion(latest.String()))
	}
}

func runUpdate(opts updateOptions) error {
	fmt.Printf("%s Versión actual: %s\n", infoStyle.Render("•"), version)

	current, currentErr := parseCurrentVersion()
	if currentErr != nil && !opts.Force && !opts.CheckOnly {
		return fmt.Errorf("esta es una compilación local (%s); usa --force para reemplazarla por el último release", version)
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), updateLookupTimeout)
	defer cancel()

	latest, err := detectLatestRelease(checkCtx, opts.Prerelease)
	if err != nil {
		if errors.Is(err, errNoPublishedRelease) {
			fmt.Println(infoStyle.Render("Todavía no hay releases publicados en GitHub."))
			return nil
		}
		return fmt.Errorf("no se pudo comprobar la última versión: %w", err)
	}

	fmt.Printf("%s Última versión: %s\n", infoStyle.Render("•"), formatReleaseVersion(latest.Version()))

	updateAvailable := current == nil || latest.GreaterThan(current.String())
	if !updateAvailable && !opts.Force {
		fmt.Println(infoStyle.Render("✅ Ya tienes la última versión instalada."))
		return nil
	}

	if opts.CheckOnly {
		if updateAvailable {
			fmt.Println(infoStyle.Render("⬆️  Hay una actualización disponible."))
		} else {
			fmt.Println(infoStyle.Render("✅ Ya tienes la última versión instalada."))
		}
		return nil
	}

	if current == nil {
		fmt.Println(infoStyle.Render("Compilación local detectada: se instalará el último release."))
	}

	if !opts.Yes {
		var confirmed bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("¿Actualizar a %s?", formatReleaseVersion(latest.Version())),
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirmed); err != nil || !confirmed {
			fmt.Println(infoStyle.Render("Actualización cancelada."))
			return nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo obtener la ruta del ejecutable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Println(infoStyle.Render("⬇️  Descargando e instalando..."))
	downloadCtx, downloadCancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	defer downloadCancel()

	updater, err := newUpdateUpdater(opts.Prerelease)
	if err != nil {
		return fmt.Errorf("no se pudo preparar el actualizador: %w", err)
	}
	if err := updater.UpdateTo(downloadCtx, latest, exe); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("no hay permisos para reemplazar %s; ejecuta Kujira CLI con permisos de administrador (sudo en Linux)", exe)
		}
		return fmt.Errorf("no se pudo instalar la actualización: %w", err)
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("✅ Actualizado a %s. Reinicia Kujira CLI para usar la nueva versión.", formatReleaseVersion(latest.Version()))))
	return nil
}

func newUpdateCommand() *cobra.Command {
	opts := updateOptions{}
	cmd := &cobra.Command{
		Use:          "update",
		Short:        "Buscar e instalar la última versión desde GitHub",
		Long:         "Comprueba los releases de Kujira CLI en GitHub y reemplaza el binario actual por la última versión disponible.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.CheckOnly, "check", false, "Solo comprobar si hay una actualización disponible")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Instalar sin pedir confirmación")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Actualizar compilaciones locales o reinstalar la misma versión")
	cmd.Flags().BoolVar(&opts.Prerelease, "prerelease", false, "Incluir versiones pre-release")
	return cmd
}

func (c *CLI) showUpdateMenu() error {
	clearScreen()
	fmt.Println(titleStyle.Render("⬆️  Actualizaciones"))
	fmt.Println()
	if err := runUpdate(updateOptions{}); err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
	}
	fmt.Println()
	fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
	fmt.Scanln()
	return nil
}
