<#
  prueba-api.ps1 — Prueba de humo de SIGO-Dental contra la API real (Etapas 1 y 2).

  Hace por comandos lo mismo que se revisa en el navegador:
    1. Login en dos pasos (contraseña + código MFA) y rechazo sin MFA
    2. Admin:      usuarios, ajustes, catálogos (caché Redis) y verificación de la auditoría
    3. Recepción:  pacientes con alerta crítica, menor sin tutor (400), agenda,
                   cita nueva, choque de sillón (409 SILLON_OCUPADO) y cancelación
    4. Odontólogo: anamnesis versionada
    5. Mensajes:   bandeja con proveedor SIMULADO

  Uso (con la base, Redis y el backend corriendo):
    cd sigo-backend
    powershell -ExecutionPolicy Bypass -File scripts\prueba-api.ps1

  El código MFA se obtiene con "go run . totp <correo>" (sólo funciona fuera de producción).
  Todos los datos son ficticios.
#>
param(
    [string]$Api = "http://127.0.0.1:3000",
    [string]$Password = "Demo-Sigo-2026"
)

$ErrorActionPreference = "Stop"
$script:fallas = 0

function Titulo($texto) { Write-Host ""; Write-Host "=== $texto ===" -ForegroundColor Cyan }
function Ok($texto) { Write-Host "  [OK] $texto" -ForegroundColor Green }
function Falla($texto) { Write-Host "  [FALLA] $texto" -ForegroundColor Red; $script:fallas++ }

# Llama a la API y devuelve @{ Status; Body }. No lanza excepción si la API responde con error:
# así se pueden comprobar también los rechazos esperados (400, 401, 403, 409).
function Api($metodo, $ruta, $token = $null, $cuerpo = $null) {
    $params = @{ Method = $metodo; Uri = "$Api$ruta"; Headers = @{}; UseBasicParsing = $true }
    if ($token) { $params.Headers["Authorization"] = "Bearer $token" }
    if ($null -ne $cuerpo) {
        $params.ContentType = "application/json; charset=utf-8"
        # Bytes UTF-8 para que las tildes lleguen bien también en Windows PowerShell 5.1.
        $params.Body = [System.Text.Encoding]::UTF8.GetBytes(($cuerpo | ConvertTo-Json -Depth 6 -Compress))
    }
    try {
        $r = Invoke-WebRequest @params
        $texto = [System.Text.Encoding]::UTF8.GetString($r.RawContentStream.ToArray())
        $json = $null
        if ($texto) { $json = $texto | ConvertFrom-Json }
        return @{ Status = [int]$r.StatusCode; Body = $json; Cache = "$($r.Headers['X-Cache'])" }
    } catch {
        $status = 0
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        $json = $null
        if ($_.ErrorDetails.Message) { try { $json = $_.ErrorDetails.Message | ConvertFrom-Json } catch { } }
        return @{ Status = $status; Body = $json }
    }
}

# Comprueba el código HTTP esperado y muestra el resultado.
function Espera($r, [int]$codigo, $texto) {
    if ($r.Status -eq $codigo) { Ok "$texto ($codigo)" }
    else {
        $msg = ""
        if ($r.Body -and $r.Body.message) { $msg = " - $($r.Body.code): $($r.Body.message)" }
        Falla "$texto -> se esperaba $codigo y llegó $($r.Status)$msg"
    }
    return $r.Body
}

# POST /api/login con el límite de 10 intentos por minuto en cuenta: si responde 429, espera y reintenta.
function Login($correo, $clave) {
    $r = Api "POST" "/api/login" $null @{ email = $correo; password = $clave }
    if ($r.Status -eq 429) {
        Write-Host "  ... límite de 10 inicios de sesión por minuto alcanzado (429); esperando 60 s" -ForegroundColor DarkGray
        Start-Sleep -Seconds 61
        $r = Api "POST" "/api/login" $null @{ email = $correo; password = $clave }
    }
    return $r
}

# Código TOTP actual del usuario (comando de desarrollo del backend).
function CodigoMFA($correo) {
    $salida = (& go run . totp $correo) -join " "
    if ($salida -match "\b(\d{6})\b") { return $Matches[1] }
    throw "No se pudo obtener el código MFA de $correo"
}

function IniciarSesion($correo) {
    $r = Login $correo $Password
    $login = Espera $r 200 "Login (1.er factor) de $correo"
    if (-not $login) { return $null }
    # Un código TOTP sólo se acepta una vez (anti-replay). Si el de esta ventana de 30 s ya se usó
    # (por ejemplo, porque acaba de ingresar en el navegador), se espera la siguiente y se reintenta.
    $mfa = Api "POST" "/api/mfa/verificar" $login.token @{ codigo = (CodigoMFA $correo) }
    if ($mfa.Status -eq 400) {
        $espera = 30 - ([DateTimeOffset]::UtcNow.ToUnixTimeSeconds() % 30) + 1
        Write-Host "  ... código MFA ya usado en esta ventana (anti-replay); esperando $espera s" -ForegroundColor DarkGray
        Start-Sleep -Seconds $espera
        $mfa = Api "POST" "/api/mfa/verificar" $login.token @{ codigo = (CodigoMFA $correo) }
    }
    $perfil = Espera $mfa 200 "MFA verificado ($($login.rol))"
    if ($perfil) { return $perfil.token }
    return $null
}

# ---------------------------------------------------------------------------------------
Titulo "Backend"
$salud = Api "GET" "/api/health"
if ($salud.Status -ne 200) { Write-Host "El backend no responde en $Api. Arránquelo con: go run ." -ForegroundColor Yellow; exit 1 }
Ok "health: $($salud.Body.status) (db $($salud.Body.db), redis $($salud.Body.redis))"

# ---------------------------------------------------------------------------------------
Titulo "1. Autenticación (RNF-05)"
$r = Login "admin@demo.sigo" "clave-incorrecta-1"
$e = Espera $r 401 "Contraseña incorrecta rechazada"
if ($e) { Ok "Mensaje genérico (no revela si el correo existe): $($e.message)" }
$r = Login "admin@demo.sigo" $Password
$sinMfa = $r.Body.token
Ok "Siguiente paso tras la contraseña: $($r.Body.siguientePaso)"
$e = Espera (Api "GET" "/api/usuarios" $sinMfa) 403 "Sin MFA no hay acceso a datos"
if ($e) { Ok "Código de error: $($e.code)" }
Espera (Api "GET" "/api/usuarios") 401 "Sin token: 401" | Out-Null

# ---------------------------------------------------------------------------------------
Titulo "2. Administrador TI"
$admin = IniciarSesion "admin@demo.sigo"
$us = Espera (Api "GET" "/api/usuarios" $admin) 200 "Usuarios"
if ($us) { Ok "$(@($us).Count) usuarios; ninguno expone hash ni secreto MFA" }
$aj = Espera (Api "GET" "/api/ajustes" $admin) 200 "Ajustes"
foreach ($a in @($aj)) { Write-Host "     $($a.clave) = $($a.valor) (rango $($a.min)-$($a.max))" }
$c1 = Api "GET" "/api/catalogos/sillones" $admin
$c2 = Api "GET" "/api/catalogos/sillones" $admin
Espera $c2 200 "Catálogo de sillones" | Out-Null
if ($c2.Cache -eq "HIT") { Ok "Caché Redis: 1.ª lectura $($c1.Cache), 2.ª lectura $($c2.Cache)" }
else { Write-Host "  [AVISO] X-Cache=$($c2.Cache): Redis no está disponible, la API funciona sin caché" -ForegroundColor Yellow }
Espera (Api "GET" "/api/pacientes" $admin) 403 "El administrador NO ve pacientes (datos clínicos)" | Out-Null
$ver = Espera (Api "POST" "/api/auditoria/verificar" $admin) 200 "Verificación de la cadena de auditoría"
if ($ver) {
    if ($ver.valida) { Ok "Cadena íntegra: $($ver.verificados) registros" } else { Falla "Cadena rota en el registro $($ver.rotaEnId)" }
}

# ---------------------------------------------------------------------------------------
Titulo "3. Recepción: pacientes y agenda (RF-01, RF-02, RF-09..11, RN-03)"
$rec = IniciarSesion "recepcion@demo.sigo"
$lista = Espera (Api "GET" "/api/pacientes?q=DEMO000101" $rec) 200 "Búsqueda de paciente (queda auditada)"
$ana = @($lista.data)[0]
if ($ana) {
    $critica = @($ana.alertas | Where-Object { $_.severidad -eq "CRITICA" })
    if ($critica.Count -gt 0) { Ok "Alerta CRÍTICA visible: $($critica[0].etiqueta)" } else { Falla "Ana debería tener alerta crítica" }
}
Espera (Api "GET" "/api/pacientes/$($ana.id)/anamnesis" $rec) 403 "Recepción NO ve el detalle de la anamnesis" | Out-Null

$menor = @{
    tipoDocumento = "CEDULA"; numeroDocumento = "PS" + (Get-Random -Maximum 99999999); nombres = "Menor"; apellidos = "Prueba Script"
    fechaNacimiento = (Get-Date).AddYears(-8).ToString("yyyy-MM-dd"); sexo = "OTRO"; emergenciaNombre = "Contacto Ficticio"
    emergenciaTelefono = "+000 0000 1234"; canalPreferido = "WHATSAPP"; autorizaMensajes = $false
}
$e = Espera (Api "POST" "/api/pacientes" $rec $menor) 400 "Menor de edad sin tutor rechazado"
if ($e) { Ok "Código: $($e.code)" }

$odo = Espera (Api "GET" "/api/catalogos/odontologos" $rec) 200 "Odontólogos"
$sil = Espera (Api "GET" "/api/catalogos/sillones" $rec) 200 "Sillones"
# Horario ficticio lejano (entre 60 y 360 días) para no chocar con los datos demo.
$inicio = (Get-Date).Date.AddDays(60 + (Get-Random -Maximum 300)).AddHours(15).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$cita = @{ pacienteId = $ana.id; odontologoId = @($odo)[0].id; sillonId = @($sil)[0].id; inicio = $inicio; duracionMin = 30; motivo = "Prueba del script" }
$nueva = Espera (Api "POST" "/api/citas" $rec $cita) 201 "Cita agendada"
if ($nueva) { Ok "Estado $($nueva.estado); $(@($nueva.notificaciones).Count) mensajes programados (confirmación, 48 h, 24 h)" }
$choque = Espera (Api "POST" "/api/citas" $rec $cita) 409 "Mismo sillón y hora rechazado (RN-03)"
if ($choque) { Ok "$($choque.code): ocupado por $($choque.extra.conflicto.paciente)" }
$cita.sillonId = @($sil)[1].id
Espera (Api "POST" "/api/citas" $rec $cita) 201 "Mismo horario en OTRO sillón permitido" | Out-Null
Espera (Api "POST" "/api/citas/$($nueva.id)/estado" $rec @{ estado = "CANCELADO" }) 400 "Cancelar sin motivo rechazado" | Out-Null
$can = Espera (Api "POST" "/api/citas/$($nueva.id)/estado" $rec @{ estado = "CANCELADO"; nota = "Prueba del script" }) 200 "Cancelada con motivo"
if ($can) { Ok "Mensajes pendientes anulados: $(@($can.notificaciones | Where-Object { $_.estado -eq 'CANCELADO' }).Count)" }
$cita.sillonId = @($sil)[0].id
Espera (Api "POST" "/api/citas" $rec $cita) 201 "La cita cancelada libera el sillón" | Out-Null

# ---------------------------------------------------------------------------------------
Titulo "4. Odontólogo: anamnesis versionada (RF-02)"
$doc = IniciarSesion "odontologo@demo.sigo"
$antes = Espera (Api "GET" "/api/pacientes/$($ana.id)/anamnesis" $doc) 200 "Historial de anamnesis"
$v = Espera (Api "POST" "/api/pacientes/$($ana.id)/anamnesis" $doc @{
    enfermedadesSistemicas = @("Hipertensión arterial controlada"); alergias = @(@{ sustancia = "Penicilina"; tipo = "MEDICAMENTO"; reaccion = "Urticaria" })
    coagulopatia = $false; anticoagulado = $false }) 201 "Nueva versión registrada"
if ($v) { Ok "Versión $($v.version) (antes había $(@($antes).Count)); las anteriores no se modifican" }

# ---------------------------------------------------------------------------------------
Titulo "5. Mensajes al paciente (RF-11) - proveedor SIMULADO"
$p = Espera (Api "POST" "/api/notificaciones/procesar" $rec) 200 "Procesar mensajes vencidos"
if ($p) { Ok "Enviados (simulado): $($p.enviados) · con error: $($p.fallidos)" }
$b = Espera (Api "GET" "/api/notificaciones" $rec) 200 "Bandeja"
if ($b) {
    if ($b.proveedor.simulado) { Ok "Proveedor '$($b.proveedor.nombre)' marcado como SIMULADO: no sale ningún mensaje real" }
    else { Falla "El proveedor debería estar marcado como simulado" }
}

# ---------------------------------------------------------------------------------------
Write-Host ""
if ($script:fallas -eq 0) { Write-Host "TODO OK" -ForegroundColor Green; exit 0 }
Write-Host "$($script:fallas) comprobación(es) fallaron" -ForegroundColor Red
exit 1
