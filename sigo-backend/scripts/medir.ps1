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
# El navegador se mide contra la build de producción (npm run build) servida por la API, como en Render.
# Con el servidor de desarrollo de Vite (5173) los tiempos no sirven: carga cientos de módulos sin empaquetar.
Push-Location ..
npm run build
Pop-Location
$env:PORT = "3001"; $env:STATIC_DIR = (Resolve-Path ..\dist).Path
$api2 = Start-Process go -ArgumentList "run", "." -PassThru -NoNewWindow
Start-Sleep -Seconds 15
Remove-Item Env:PORT, Env:STATIC_DIR
Push-Location ..
$env:SIGO_URL = "http://localhost:3001"
try {
  npx playwright test --project=chromium --project=firefox --project=webkit --project=edge --project=tablet-10.5 --project=fullhd --project=celular
} finally {
  Pop-Location
  Get-NetTCPConnection -LocalPort 3001 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force }
  if (!$api2.HasExited) { Stop-Process -Id $api2.Id -Force }
}
