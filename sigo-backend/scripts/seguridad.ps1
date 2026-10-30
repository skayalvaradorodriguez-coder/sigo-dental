# Análisis de seguridad (SRS 8.2): SAST, dependencias vulnerables y DAST. Ejecutar dentro de sigo-backend.
# Requiere internet; el DAST requiere Docker Desktop y la API corriendo en el puerto 3000 con STATIC_DIR=../dist.
$ErrorActionPreference = "Continue"
Write-Host "=== SAST: gosec ===" -ForegroundColor Cyan
go install github.com/securego/gosec/v2/cmd/gosec@v2.21.4
& "$(go env GOPATH)\bin\gosec.exe" -quiet ./...
Write-Host "`n=== Dependencias de Go: govulncheck ===" -ForegroundColor Cyan
go install golang.org/x/vuln/cmd/govulncheck@latest
& "$(go env GOPATH)\bin\govulncheck.exe" ./...
Write-Host "`n=== Dependencias del frontend: npm audit (producción) ===" -ForegroundColor Cyan
Push-Location ..
npm audit --omit=dev
Pop-Location
Write-Host "`n=== DAST: OWASP ZAP (baseline) contra http://localhost:3000 ===" -ForegroundColor Cyan
docker run --rm -v "${PWD}:/zap/wrk:rw" -t ghcr.io/zaproxy/zaproxy:stable zap-baseline.py `
  -t http://host.docker.internal:3000 -r zap-informe.html -I
Write-Host "Informe: sigo-backend\zap-informe.html (los avisos FAIL/WARN se revisan uno por uno)"
