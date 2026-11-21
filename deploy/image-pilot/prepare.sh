#!/bin/sh
# Run in the staging directory produced by import.py before installing the library.
set -eu
while IFS= read -r asset; do
    file="library/$asset"
    name=${asset##*/}
    printf '%s  %s\n' "${name%.*}" "$file" | sha256sum --check --status
    dest="library/preview/$asset.webp"
    mkdir -p "$(dirname "$dest")"
    vips thumbnail "$file" "$dest.tmp.webp[Q=84,strip]" 780 --height 1040 --crop attention
    mv "$dest.tmp.webp" "$dest"
done < manifest.txt
