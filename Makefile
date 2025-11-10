.PHONY: build build-windows build-linux build-macos

# Versión y metadatos
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT = $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build para Linux
dist/linux/kujira-cli:
	GOOS=linux GOARCH=amd64 go build $(VERSION_FLAGS) -o $@

# Build para Windows con icono y versión
ICON_SRC=assets/icon.ico
GOVERSIONINFO_BIN=$(shell go env GOPATH)/bin/goversioninfo

# Información de versión para los ejecutables
VERSION_FLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildDate=$(BUILD_DATE)' -X 'main.gitCommit=$(GIT_COMMIT)'"
WINDOWS_FLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildDate=$(BUILD_DATE)' -X 'main.gitCommit=$(GIT_COMMIT)'"

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
dist/windows/kujira-cli.exe: versioninfo.syso
	@echo "Compilando para Windows con icono y versión..."
	@GOOS=windows GOARCH=amd64 go build $(WINDOWS_FLAGS) -o $@
	@# Limpiar archivo .syso después de la compilación
	@rm -f versioninfo.syso

# Instalar dependencias para Windows
deps-windows:
	go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest

# Build para macOS
dist/macos/kujira-cli:
	GOOS=darwin GOARCH=amd64 go build $(VERSION_FLAGS) -o $@

# Crear directorios de salida
prepare-dist:
	mkdir -p dist/linux dist/windows dist/macos

# Build para todas las plataformas
build-all: prepare-dist deps-windows dist/linux/kujira-cli dist/windows/kujira-cli.exe dist/macos/kujira-cli

# Build local (sistema actual)
build:
	go build $(LDFLAGS) -o kujira-cli

# Limpiar archivos generados
clean:
	rm -rf dist kujira-cli kujira-cli.exe kujira-cli.syso

# Instalar en el sistema
install: build
	sudo cp kujira-cli /usr/local/bin/


