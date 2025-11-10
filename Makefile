.PHONY: build build-windows build-linux build-macos

# Versión y metadatos
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT = $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build para Linux
dist/linux/kujira-cli:
	GOOS=linux GOARCH=amd64 go build $(VERSION_FLAGS) -o $@

# Build para Windows con icono
ICON_SRC=assets/icon.ico
RSRC_BIN=$(shell go env GOPATH)/bin/rsrc

# Información de versión para los ejecutables
VERSION_FLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildDate=$(BUILD_DATE)' -X 'main.gitCommit=$(GIT_COMMIT)'"
WINDOWS_FLAGS=-ldflags "-H=windowsgui -X 'main.version=$(VERSION)' -X 'main.buildDate=$(BUILD_DATE)' -X 'main.gitCommit=$(GIT_COMMIT)'"

dist/windows/kujira-cli.exe: assets/kujira-cli.manifest $(ICON_SRC)
	@echo "Compilando para Windows con icono..."
	# Primero compilamos sin el icono
	@GOOS=windows GOARCH=amd64 go build $(WINDOWS_FLAGS) -o $@
	# Luego añadimos el icono
	@if [ -f "$(RSRC_BIN)" ]; then \
		echo "Añadiendo icono al ejecutable..."; \
		$(RSRC_BIN) -manifest assets/kujira-cli.manifest -ico $(ICON_SRC) -o kujira-cli.syso; \
		# Recompilamos con el icono \
		GOOS=windows GOARCH=amd64 go build $(WINDOWS_FLAGS) -o $@; \
		rm -f kujira-cli.syso; \
	else \
		echo "ADVERTENCIA: No se encontró rsrc. Asegúrate de que esté instalado y en tu PATH."; \
		echo "Instálalo con: go install github.com/akavel/rsrc@latest"; \
		echo "Y asegúrate de que $(go env GOPATH)/bin está en tu PATH"; \
		echo "Continuando sin el icono..."; \
	fi

# Build para macOS
dist/macos/kujira-cli:
	GOOS=darwin GOARCH=amd64 go build $(VERSION_FLAGS) -o $@

# Instalar dependencias para Windows (rsrc)
deps-windows:
	go install github.com/akavel/rsrc@latest

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

# Crear un archivo de manifiesto para Windows
assets/kujira-cli.manifest:
	@mkdir -p assets
	@echo '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">\
  <assemblyIdentity \
    version="$(VERSION)" \
    processorArchitecture="*" \
    name="Kujira.CLI" \
    type="win32" \
  />\
  <description>Kujira CLI - Herramienta de línea de comandos para anime</description>\
  <dependency>\
    <dependentAssembly>\
      <assemblyIdentity \
        type="win32" \
        name="Microsoft.Windows.Common-Controls" \
        version="6.0.0.0" \
        processorArchitecture="*" \
        publicKeyToken="6595b64144ccf1df" \
        language="*" \
      />\
    </dependentAssembly>\
  </dependency>\
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">\
    <security>\
      <requestedPrivileges>\
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>\
      </requestedPrivileges>\
    </security>\
  </trustInfo>\
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">\
    <application>\
      <!-- Windows 10 and Windows 11 -->\
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>\
      <!-- Windows 8.1 -->\
      <supportedOS Id="{1f676c76-80e1-4239-95bb-83d0f6d0da78}"/>\
      <!-- Windows 8 -->\
      <supportedOS Id="{4a2f28e3-53b9-4441-ba9c-d69d4a4a6e38}"/>\
      <!-- Windows 7 -->\
      <supportedOS Id="{35138b9a-5d96-4fbd-8e2d-a2440225f93a}"/>\
    </application>\
  </compatibility>\
</assembly>' > $@

# # Descargar un icono de ejemplo si no existe
# assets/icon.ico:
# 	@mkdir -p assets
# 	@echo "Descargando icono de ejemplo..."
# 	@if ! curl -s -L -o $@ https://raw.githubusercontent.com/github/explore/80688e429a7d4ef2fca1e82350fe8e3517d3494d/topics/go/go.png; then \
# 		echo "No se pudo descargar el icono. Por favor, coloca manualmente un archivo icon.ico en la carpeta assets/"; \
# 		exit 1; \
# 	fi
