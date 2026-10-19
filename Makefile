.PHONY: build build-all build-windows build-linux icons install uninstall prepare-dist release-assets clean deps-windows FORCE

# Versión y metadatos
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT = $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build para Linux
dist/linux/kujira-cli: FORCE
	@mkdir -p dist/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(VERSION_FLAGS) -o $@

dist/linux/kujira-cli-arm64: FORCE
	@mkdir -p dist/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(VERSION_FLAGS) -o $@

# Build para Windows con icono y versión
ICON_SRC=assets/icon.ico
GOVERSIONINFO_BIN=$(shell go env GOPATH)/bin/goversioninfo

# Información de versión para los ejecutables
VERSION_FLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildDate=$(BUILD_DATE)' -X 'main.gitCommit=$(GIT_COMMIT)'"

# Generar archivo .syso con la información de versión y el ícono
versioninfo.syso: versioninfo.json $(ICON_SRC)
	@if [ -f "$(GOVERSIONINFO_BIN)" ]; then \
		echo "Generando archivo de recursos de Windows..."; \
		$(GOVERSIONINFO_BIN) -icon $(ICON_SRC) -o $@; \
	else \
		echo "ERROR: goversioninfo no está instalado. Por favor instálalo con:"; \
		echo "go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"; \
		exit 1; \
	fi

# Construir para Windows
dist/windows/kujira-cli.exe: versioninfo.syso FORCE
	@echo "Compilando para Windows con icono y versión..."
	@mkdir -p dist/windows
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(VERSION_FLAGS) -o $@
	@# Limpiar archivo .syso después de la compilación
	@rm -f versioninfo.syso

# Instalar dependencias para Windows
deps-windows:
	go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

# Crear directorios de salida
prepare-dist:
	mkdir -p dist/linux dist/windows dist/release

# Build para todas las plataformas
build-all: prepare-dist deps-windows build-linux build-windows

build-linux: dist/linux/kujira-cli dist/linux/kujira-cli-arm64
build-windows: dist/windows/kujira-cli.exe

# Generar los iconos desde assets/icon.svg
icons:
	./scripts/generate-icons.sh

# Preparar los binarios con los nombres que espera el auto-update
release-assets: prepare-dist build-linux build-windows
	cp dist/linux/kujira-cli dist/release/kujira-cli_linux_amd64
	cp dist/linux/kujira-cli-arm64 dist/release/kujira-cli_linux_arm64
	cp dist/windows/kujira-cli.exe dist/release/kujira-cli_windows_amd64.exe
	cd dist/release && sha256sum kujira-cli_linux_amd64 kujira-cli_linux_arm64 kujira-cli_windows_amd64.exe > checksums.txt

# Build local (sistema actual)
build:
	go build $(VERSION_FLAGS) -o kujira-cli

# Instalar en el sistema (usar sudo si es necesario)
PREFIX ?= /usr/local
DESTDIR ?=
install: build
	install -Dm755 kujira-cli $(DESTDIR)$(PREFIX)/bin/kujira-cli
	install -Dm644 assets/icon-512.png $(DESTDIR)$(PREFIX)/share/icons/hicolor/512x512/apps/kujira-cli.png
	install -Dm644 assets/icon.svg $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/kujira-cli.svg
	install -Dm644 assets/kujira-cli.desktop $(DESTDIR)$(PREFIX)/share/applications/kujira-cli.desktop
	@if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database $(DESTDIR)$(PREFIX)/share/applications >/dev/null 2>&1 || true; fi
	@if command -v gtk-update-icon-cache >/dev/null 2>&1; then gtk-update-icon-cache -f -t $(DESTDIR)$(PREFIX)/share/icons/hicolor >/dev/null 2>&1 || true; fi

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/kujira-cli
	rm -f $(DESTDIR)$(PREFIX)/share/icons/hicolor/512x512/apps/kujira-cli.png
	rm -f $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/kujira-cli.svg
	rm -f $(DESTDIR)$(PREFIX)/share/applications/kujira-cli.desktop

# Limpiar archivos generados
clean:
	rm -rf dist kujira-cli kujira-cli.exe versioninfo.syso kujira-cli.syso

# Fuerza la reconstrucción de los binarios de release
FORCE:
