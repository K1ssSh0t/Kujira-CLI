# Kujira CLI

Cliente de línea de comandos interactivo para buscar, ver y llevar el seguimiento de anime desde distintos proveedores, con historial local, biblioteca y sincronización con MyAnimeList.

## Características

- Cuatro proveedores: `animeflv`, `tioanime`, `animeav1` y `monoschinos`.
- Reproducción con `mpv`, incluyendo extractores para Voe, StreamTape, Mp4Upload y Uqload.
- Últimos estrenos, búsqueda, detalle de episodios y servidores por idioma (SUB/DUB/LAT).
- Historial de reproducción y biblioteca personal.
- Sincronización con MyAnimeList (OAuth2 a través del backend de Kujira) y auto-sync del episodio reproducido.
- Caché local de las respuestas de los proveedores durante 1 hora.
- Aviso de nuevas versiones y actualización desde GitHub Releases.

## Requisitos

- **Sistema**: Linux (`amd64`/`arm64`) o Windows (`amd64`). En macOS solo se puede compilar desde el código fuente.
- **mpv** instalado y disponible en el `PATH`; es el reproductor que usa la aplicación.
- **Go 1.24.5 o superior** solo si vas a compilar.
- **make** y **goversioninfo** solo para compilar el binario de Windows con icono y metadatos (el workflow de CI instala `goversioninfo` automáticamente).

## Instalación

### Desde GitHub Releases

En Linux:

```bash
curl -L -O https://github.com/K1ssSh0t/Kujira-CLI/releases/latest/download/kujira-cli_linux_amd64
curl -L -O https://github.com/K1ssSh0t/Kujira-CLI/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing
chmod +x kujira-cli_linux_amd64
sudo mv kujira-cli_linux_amd64 /usr/local/bin/kujira-cli
```

Para ARM64 usa `kujira-cli_linux_arm64`.

En Windows, descarga `kujira-cli_windows_amd64.exe` desde la página del release y colócalo en una carpeta de tu `PATH` (por ejemplo, renombrado como `kujira-cli.exe`).

### Desde el código fuente

```bash
make build          # genera ./kujira-cli con versión derivada de git
sudo make install   # instala binario, iconos y entrada de escritorio en /usr/local
```

`make install` acepta `PREFIX` y `DESTDIR`:

```bash
make install PREFIX=$HOME/.local
```

Para desinstalar:

```bash
sudo make uninstall
```

## Uso

```bash
kujira-cli                       # menú interactivo
kujira-cli search "one piece" -p animeflv
kujira-cli --version
kujira-cli update                # buscar e instalar la última versión
kujira-cli update --check        # solo comprobar si hay una nueva versión
```

Opciones globales:

| Flag | Descripción |
|---|---|
| `-p, --provider` | Proveedor a usar: `animeflv`, `tioanime`, `animeav1`, `monoschinos` |
| `--no-update-check` | No comprobar actualizaciones al iniciar |
| `-v, --version` | Mostrar versión, fecha de compilación y commit |

En el menú interactivo también existe la opción **Buscar actualizaciones**.

## Actualizaciones automáticas

Al arrancar el menú interactivo, Kujira CLI consulta GitHub Releases como máximo **una vez cada 24 horas** y muestra un aviso si hay una versión más nueva. No reemplaza el binario por sí solo: el usuario ejecuta `kujira-cli update` (o entra a *Buscar actualizaciones*) para instalarla.

- Solo se consideran releases **publicados** (no drafts) y **estables**; las pre-releases requieren `--prerelease`.
- La comprobación se puede desactivar con `--no-update-check` o con la variable de entorno `KUJIRA_NO_UPDATE_CHECK=1`.
- Los builds locales con versión `dev` o sin versión semver no se actualizan solos; requieren `--force`.
- Si el binario está en una ruta protegida, ejecuta la actualización con permisos de administrador (`sudo` en Linux).

El proceso completo para publicar una versión y hacer que el actualizador la detecte está en **[docs/RELEASING.md](docs/RELEASING.md)**.

## Datos del usuario

Todo se guarda en `~/.kujira_cli/`:

| Ruta | Contenido |
|---|---|
| `cache/` | Respuestas de los proveedores (válidas 1 hora) |
| `history.json` | Historial de reproducción |
| `library.json` | Biblioteca personal |
| `sync_config.json` | Tokens y configuración de MyAnimeList (permisos `0600`) |
| `update_check.json` | Última comprobación de actualizaciones |

## Sincronización con MyAnimeList

La autenticación se hace con OAuth2 a través del backend de Kujira, así que no necesitas crear tu propia aplicación de API de MAL.

1. Entra a **Sincronizar con MAL → Configurar MAL**.
2. Abre en el navegador el enlace que muestra la aplicación.
3. Autoriza la aplicación; el callback llega a `http://localhost:8080/callback`, por lo que **el puerto 8080 debe estar libre**.
4. Al volver a la terminal, la configuración queda guardada.

Desde ese menú puedes probar la conexión, sincronizar biblioteca e historial, hacer una sincronización completa y activar o desactivar el auto-sync.

## Desarrollo

```bash
go test ./...            # tests
go vet ./...             # análisis estático
make build               # build local
make build-all           # Linux amd64/arm64 + Windows amd64
make release-assets      # binarios con los nombres que espera el auto-update
make icons               # regenerar iconos desde assets/icon.svg
make clean               # limpiar artefactos
```

Estructura relevante:

- `main.go`: menús, proveedores, caché, historial, biblioteca y reproducción.
- `sync/`: integración con MyAnimeList y lógica de sincronización.
- `extractors/`: extracción de enlaces de los servidores de video.
- `update.go`: comprobación e instalación de actualizaciones.
- `.github/workflows/build.yml`: compilación y publicación automática de releases.
- `Makefile`: builds locales, assets de release e instalación.

## Publicar una nueva versión

En resumen: haz push a `main` y el workflow calcula la versión, compila y publica el release. Para cambios de versión **minor** o **major** hay que editar `BASE_VERSION` en `.github/workflows/build.yml`. Nunca crees el tag a mano.

Lee la guía completa en **[docs/RELEASING.md](docs/RELEASING.md)**.
