#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/build"
mkdir -p "$OUT"

echo "Building rotator (linux/arm64)"
GOOS=linux GOARCH=arm64 go build -o "$OUT/rotator" ./cmd/rotator
pushd "$OUT" > /dev/null

# When creating the package we want only a single executable named 'bootstrap' inside the zip
# to avoid leaving both 'rotator' and 'bootstrap' in the function package. We'll temporarily
# rename 'rotator' to 'bootstrap' when using the system zip, and restore it afterwards.
TMP_RENAMED=false
cleanup() {
	if [ "$TMP_RENAMED" = true ]; then
		if [ -f "bootstrap" ] && [ ! -f "rotator" ]; then
			mv bootstrap rotator || true
		fi
	fi
}
trap cleanup EXIT

if [ ! -f "rotator" ]; then
	echo "Error: built binary 'rotator' not found in $OUT"
	popd > /dev/null
	exit 1
fi

if command -v zip >/dev/null 2>&1; then
	echo "Creating zip with zip (temporary rename to bootstrap)"
	mv rotator bootstrap
	TMP_RENAMED=true
	zip -r rotator.zip bootstrap
	# restore
	mv bootstrap rotator
	TMP_RENAMED=false
elif command -v python3 >/dev/null 2>&1 || command -v python >/dev/null 2>&1; then
	PY=$(command -v python3 || command -v python)
	echo "Creating zip with $PY (writing single entry 'bootstrap' from rotator)"
	$PY - <<PY
import zipfile, os
zf=zipfile.ZipFile('rotator.zip','w', zipfile.ZIP_DEFLATED)
if os.path.exists('rotator'):
		with open('rotator','rb') as f:
				data=f.read()
		info=zipfile.ZipInfo('bootstrap')
		info.external_attr = (0o755) << 16
		zf.writestr(info, data)
zf.close()
PY
elif command -v powershell.exe >/dev/null 2>&1; then
	echo "Creating zip with PowerShell (temporary rename)"
	powershell.exe -NoProfile -Command "Rename-Item -Path 'rotator' -NewName 'bootstrap'; Compress-Archive -Path 'bootstrap' -DestinationPath 'rotator.zip' -Force; Rename-Item -Path 'bootstrap' -NewName 'rotator'"
	echo "Warning: PowerShell zip may not preserve unix permissions. Use WSL or install 'zip' or 'python' for correct permissions."
else
	echo "Error: no 'zip' command, no python, and no powershell available to create zip."
	echo "Install 'zip' (e.g., apt-get install zip / choco install zip) or use WSL."
	popd > /dev/null
	exit 1
fi

trap - EXIT
cleanup

popd > /dev/null

echo "Lambda package: $OUT/rotator.zip"
