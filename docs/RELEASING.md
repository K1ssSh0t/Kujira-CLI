# Publicar una nueva versión

Esta guía describe cómo se publica una versión de Kujira CLI y qué debe cumplir el release para que el actualizador automático de la aplicación lo detecte.

## Resumen rápido

1. Haz commit de tus cambios en `main`.
2. Push a `main`.
3. El workflow **Build and Release** calcula la versión, compila Linux amd64, Linux arm64 y Windows amd64, y publica el release en GitHub.
4. Los usuarios ven el aviso en el menú principal (como máximo una vez cada 24 h) y ejecutan `kujira-cli update`.

No hay que crear tags ni releases a mano. El tag lo crea el workflow.

## Cómo funciona el pipeline

El workflow está en `.github/workflows/build.yml` y se dispara con:

- `push` a la rama `main`.
- `workflow_dispatch` manual (solo publica si se ejecuta sobre `main`).

Jobs:

| Job | Qué hace |
|---|---|
| `prepare` | Calcula la siguiente versión a partir del último tag y de `BASE_VERSION`. |
| `build-linux` | Corre `go test ./...` y `go vet ./...`, compila Linux amd64 y arm64 con la versión en `-ldflags`. |
| `build-windows` | Compila Windows amd64, sincroniza `versioninfo.json` en el runner e instala `goversioninfo`. |
| `create-release` | Renombra los binarios, genera `checksums.txt` y publica el release con su tag. |

Flujo:

```
push a main
   │
   ├─ prepare: versión = último tag + 1 patch (o BASE_VERSION)
   │
   ├─ build-linux   ─┐
   ├─ build-windows ─┤
   │                 └─ create-release: tag vX.Y.Z + assets + checksums.txt
   │
   └─ el updater de la app detecta el release en la siguiente comprobación
```

La concurrencia está serializada con el grupo `release-main`, así que dos pushes seguidos generan dos releases en orden, no uno pisando al otro.

Cada push a `main` publica una versión, incluso si solo cambian el README o la documentación. Si no quieres publicar en ese push, incluye `[skip ci]` en el mensaje del commit.

## Reglas de versionado

Las versiones siguen **SemVer** con el formato `vMAJOR.MINOR.PATCH` (por ejemplo `v1.5.1`). El tag debe tener los tres componentes numéricos.

La versión se calcula así:

1. Se busca el tag más alto que cumpla `v[0-9]*.[0-9]*.[0-9]*`.
2. Se compara con `BASE_VERSION` (definido en `env:` del workflow).
3. Si no hay tags, o el último tag es menor que la base, se publica `BASE_VERSION`.
4. En caso contrario, se incrementa el patch en 1.

Ejemplos con `BASE_VERSION: "1.5.0"`:

| Último tag | Versión publicada |
|---|---|
| *(ninguno)* | `v1.5.0` |
| `v1.0.0` | `v1.5.0` |
| `v1.4.2` | `v1.5.0` |
| `v1.5.0` | `v1.5.1` |
| `v1.6.2` | `v1.6.3` |

### Patch

No hagas nada especial: cada push a `main` incrementa el patch automáticamente.

### Minor o major

Edita `BASE_VERSION` en `.github/workflows/build.yml` y haz push a `main`. Ese mismo push publica la nueva base.

```yaml
env:
  BASE_VERSION: "1.6.0"
```

Después de publicar `1.6.0`, los siguientes pushes generan `1.6.1`, `1.6.2`, etc. Para saltar a `1.7.0` o `2.0.0`, vuelve a subir `BASE_VERSION`.

Reglas importantes:

- **Nunca bajes** `BASE_VERSION`; el tag más alto siempre manda.
- **Nunca crees el tag a mano.** Si etiquetas `v1.6.0` y luego pusheas, el workflow verá ese tag y publicará `v1.6.1` en lugar de `v1.6.0`.
- **No uses tags con pre-release** (`v1.6.0-rc.1`). El workflow los detecta por el glob pero falla al validar que el patch sea numérico. Las pre-releases tendrían que publicarse manualmente y los usuarios necesitan `--prerelease` para verlas.

## Antes de publicar

- [ ] Los cambios están en `main` (la rama `master` **no** dispara el workflow).
- [ ] `go test ./...` y `go vet ./...` pasan en local.
- [ ] Si es un cambio minor/major, `BASE_VERSION` está actualizado.
- [ ] Si cambió la URL del repositorio o el nombre del proyecto, actualiza `updateRepoSlug` en `update.go` y los nombres de los assets en el workflow.
- [ ] Si quieres que el binario de Windows local tenga metadatos correctos, actualiza `versioninfo.json` (en CI se reescribe solo en el runner; el archivo del repo no se modifica).

## Verificar el release

Con GitHub CLI:

```bash
gh release view v1.5.1
gh release view v1.5.1 --json tagName,isDraft,isPrerelease,assets \
  --jq '{tag: .tagName, draft: .isDraft, prerelease: .isPrerelease, assets: [.assets[].name]}'
```

Con la API pública:

```bash
curl -s https://api.github.com/repos/K1ssSh0t/Kujira-CLI/releases/latest \
  | jq '{tag: .tag_name, draft: .draft, prerelease: .prerelease, assets: [.assets[].name]}'
```

El release debe cumplir:

- `draft: false` y `prerelease: false`.
- Tag con formato `vX.Y.Z`.
- Exactamente estos assets (los nombres importan):

| Plataforma | Asset |
|---|---|
| Linux amd64 | `kujira-cli_linux_amd64` |
| Linux arm64 | `kujira-cli_linux_arm64` |
| Windows amd64 | `kujira-cli_windows_amd64.exe` |
| Checksums | `checksums.txt` |

Verifica también las checksums descargando los assets con el mismo nombre:

```bash
curl -L -O https://github.com/K1ssSh0t/Kujira-CLI/releases/download/v1.5.1/checksums.txt
curl -L -O https://github.com/K1ssSh0t/Kujira-CLI/releases/download/v1.5.1/kujira-cli_linux_amd64
sha256sum -c checksums.txt --ignore-missing
```

## Qué necesita el updater para detectar la versión

El actualizador vive en `update.go` y usa `github.com/creativeprojects/go-selfupdate` contra `K1ssSh0t/Kujira-CLI`. Para que funcione, el release debe cumplir **todo** esto:

1. **Tag semver** con tres números. Se compara por versión, no por fecha.
2. **Release publicado**, no draft.
3. **No marcado como pre-release** (el canal estable lo ignora).
4. **Asset cuyo nombre coincida** con `kujira-cli_<os>_<arch>`; en Windows se acepta el sufijo `.exe`. Los separadores `_` y `-` son válidos, pero mantén el formato actual.
5. **`checksums.txt` presente** en el mismo release, con una línea SHA-256 por asset y el nombre exacto del archivo. Si falta, el updater no puede validar y descarta el release.
6. **El binario instalado debe tener una versión semver compilada** mediante `-ldflags "-X main.version=..."`. Los builds locales con `dev` requieren `--force`.
7. **Acceso a la API de GitHub**. Sin autenticación el límite es de 60 peticiones por hora e IP; el chequeo de arranque lo ignora silenciosamente si falla.

Comportamiento del updater:

- La comprobación de arranque ocurre solo en el menú interactivo, no en `kujira-cli search`.
- Se ejecuta como máximo una vez cada 24 h. La caché está en `~/.kujira_cli/update_check.json`.
- El timeout de la comprobación de arranque es de 3 segundos; si la red es lenta, se guarda "sin novedad" hasta el día siguiente.
- `kujira-cli update` instala; el aviso por sí solo no reemplaza el binario.
- `--prerelease` permite ver pre-releases.
- `KUJIRA_NO_UPDATE_CHECK=1` o `--no-update-check` desactivan la comprobación.

## Probar el updater

Desde una versión anterior instalada:

```bash
kujira-cli update --check
# Debe mostrar la última versión y "Hay una actualización disponible"
kujira-cli update
```

Desde un build local (`version = "dev"`), `--check` funciona igual, pero para instalar hay que forzar:

```bash
kujira-cli update --check
kujira-cli update --force
```

Para forzar la comprobación de arranque sin esperar 24 h:

```bash
rm -f ~/.kujira_cli/update_check.json
kujira-cli
```

Para verificar que el asset descargado es el correcto sin instalar nada:

```bash
kujira-cli update --check
```

## Problemas comunes

| Síntoma | Causa probable | Solución |
|---|---|---|
| "Todavía no hay releases publicados en GitHub." | No hay ningún release con tag y asset válidos para tu OS/arquitectura. | Revisa que el release no sea draft/pre-release, que el tag sea `vX.Y.Z` y que exista el asset con el nombre correcto. |
| "no se pudo comprobar la última versión: ... validation asset not found" | Falta `checksums.txt` o no contiene el asset. | Vuelve a subir `checksums.txt` generado con `sha256sum` de los tres binarios. |
| "no se pudo comprobar la última versión: ..." (red/rate limit) | Sin conexión o límite de la API de GitHub. | Reintenta más tarde; el arranque lo ignora y reintenta al día siguiente. |
| El aviso no aparece | Caché de 24 h, versión `dev`, timeout de 3 s o release prerelease. | Borra `~/.kujira_cli/update_check.json`, usa un binario con versión semver y verifica el release. |
| "no hay permisos para reemplazar ..." | El binario está en `/usr/local/bin` y es propiedad de root. | Ejecuta `sudo kujira-cli update`. |
| Se publicó la versión equivocada | Se creó un tag manual o se re-ejecutó el workflow después de publicar. | Publica un patch con el fix; no reutilices el tag. |
| El push no publicó nada | Se hizo push a `master` en vez de `main`. | Cambia a `main` y vuelve a empujar. |
| Windows no actualiza | Antivirus, binario en uso o asset `.exe` ausente. | Cierra la app, revisa el release y usa una ruta escribible. |
| macOS no recibe actualizaciones | El workflow no compila darwin. | Compila desde fuente o añade un job de macOS y su asset. |

## Release defectuoso y rollback

- **Lo normal es publicar un patch con el fix.** El updater solo actualiza hacia arriba, así que una versión nueva siempre reemplaza a la mala.
- Si el release es reciente y quieres retirarlo:

  ```bash
  gh release delete v1.5.1 --yes --cleanup-tag
  ```

  Esto elimina el release y el tag, y el updater volverá a ver la versión anterior. **Ojo:** los usuarios que ya instalaron la versión mala no bajan de versión automáticamente; tienen que reinstalar a mano un binario anterior.

- Si solo está mal un asset, vuelve a subirlo junto con un `checksums.txt` actualizado. No cambies el tag.
- **No reutilices un tag ya publicado** para corregir: crea otro patch. Reutilizarlo puede provocar que algunos clientes ya hayan cacheado la versión y no vuelvan a descargarla.

## Archivos implicados

| Archivo | Rol |
|---|---|
| `.github/workflows/build.yml` | Cálculo de versión, builds y publicación del release. |
| `Makefile` | Builds, nombres de assets y `checksums.txt` para releases manuales. |
| `update.go` | Repositorio, detección, descarga e instalación de actualizaciones. |
| `versioninfo.json` | Metadatos del ejecutable de Windows (CI lo reescribe en el runner). |
| `update_test.go` | Tests de versiones, aviso y caché. |
