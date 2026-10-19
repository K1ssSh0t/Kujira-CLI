#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if command -v magick >/dev/null 2>&1; then
    MAGICK="magick"
elif command -v convert >/dev/null 2>&1; then
    MAGICK="convert"
else
    echo "ERROR: se necesita ImageMagick (magick o convert) para generar los iconos." >&2
    exit 1
fi

if command -v rsvg-convert >/dev/null 2>&1; then
    rsvg-convert -w 1024 -h 1024 assets/icon.svg -o assets/icon.png
else
    "$MAGICK" -background none assets/icon.svg -resize 1024x1024 assets/icon.png
fi

"$MAGICK" assets/icon.png -resize 512x512 assets/icon-512.png
"$MAGICK" assets/icon.png -define icon:auto-resize=256,128,64,48,32,24,16 assets/icon.ico

echo "Iconos generados en assets/"
