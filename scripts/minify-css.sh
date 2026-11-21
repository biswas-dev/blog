#!/usr/bin/env bash
# Rebuild static/css/theme.min.css from theme.css. Run after editing theme.css, then bump the
# ?v= on the stylesheet link in templates/tailwind.gohtml (the file is cached for a year).
set -euo pipefail
cd "$(dirname "$0")/.."
npx -y esbuild@0.25.10 static/css/theme.css --minify --loader:.css=css --log-level=warning > static/css/theme.min.css
wc -c static/css/theme.min.css
