# Mediciones de la Etapa 7 (RNF-01, RNF-03, RNF-07, RNF-08). Ejecutar dentro de sigo-backend con la API
# corriendo (go run .) y los datos de demostración cargados. Nada de esto corre en producción.
param([string]$Api = "http://localhost:3000", [int]$Usuarios = 150, [int]$Segundos = 60)
$ErrorActionPreference = "Stop"
Write-Host "=== Paciente de volumen (10 años de historia) ===" -ForegroundColor Cyan
go run . volumen
Write-Host "`n=== RNF-01: tiempos del expediente por la API ===" -ForegroundColor Cyan
go run . medir $Api 30
Write-Host "`n=== RNF-03: $Usuarios usuarios concurrentes durante $Segundos s ===" -ForegroundColor Cyan
go run . carga $Api $Usuarios $Segundos
Write-Host "`n=== RNF-01 en el navegador, RNF-07 y RNF-08 (Playwright) ===" -ForegroundColor Cyan
Write-Host "Primera vez: npx playwright install chromium firefox webkit" -ForegroundColor DarkGray
Push-Location ..
$env:SIGO_URL = "http://localhost:5173"   # o http://localhost:3000 si la API sirve la build (STATIC_DIR)
npx playwright test --project=chromium --project=firefox --project=webkit --project=edge --project=tablet-10.5 --project=fullhd --project=celular
Pop-Location
