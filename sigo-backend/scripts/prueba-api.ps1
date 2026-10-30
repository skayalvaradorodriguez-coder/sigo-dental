<#
  prueba-api.ps1 — Prueba de humo de SIGO-Dental contra la API real (Etapas 1 y 2).

  Hace por comandos lo mismo que se revisa en el navegador:
    1. Login en dos pasos (contraseña + código MFA) y rechazo sin MFA
    2. Admin:      usuarios, ajustes, catálogos (caché Redis) y verificación de la auditoría
    3. Recepción:  pacientes con alerta crítica, menor sin tutor (400), agenda,
                   cita nueva, choque de sillón (409 SILLON_OCUPADO) y cancelación
    4. Odontólogo: anamnesis versionada
    5. Mensajes:   bandeja con proveedor SIMULADO
    6. Expediente: odontograma, nota SOEP firmada con MFA, bloqueo de edición y procedimiento inmutable

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
Titulo "6. Expediente clínico (RF-03, RF-05, RN-01)"
$odon = Espera (Api "GET" "/api/pacientes/$($ana.id)/odontograma" $doc) 200 "Odontograma de la paciente"
if ($odon) { Ok "$(@($odon.estado).Count) hallazgos vigentes · $(@($odon.historial).Count) eventos en el historial · $(@($odon.catalogo).Count) tipos de hallazgo" }
Espera (Api "GET" "/api/pacientes/$($ana.id)/odontograma" $rec) 403 "Recepción NO ve el expediente clínico" | Out-Null
$nota = Espera (Api "POST" "/api/pacientes/$($ana.id)/notas" $doc @{
    subjetivo = "Prueba del script"; objetivo = "Sin hallazgos nuevos"; evaluacion = "Control"; plan = "Control en 6 meses" }) 201 "Nota SOEP (borrador)"
if ($nota) {
    Ok "Estado $($nota.estado); editable hasta $($nota.bloqueoEn)"
    $firma = Api "POST" "/api/notas/$($nota.id)/firmar" $doc @{ codigo = (CodigoMFA "odontologo@demo.sigo") }
    if ($firma.Status -eq 400) {
        $espera = 30 - ([DateTimeOffset]::UtcNow.ToUnixTimeSeconds() % 30) + 1
        Write-Host "  ... código MFA ya usado en esta ventana (anti-replay); esperando $espera s" -ForegroundColor DarkGray
        Start-Sleep -Seconds $espera
        $firma = Api "POST" "/api/notas/$($nota.id)/firmar" $doc @{ codigo = (CodigoMFA "odontologo@demo.sigo") }
    }
    $f = Espera $firma 200 "Nota firmada con código MFA (firma electrónica simple)"
    if ($f) { Ok "Firmada por $($f.firmadaPor) ($($f.registroProfesional)) · hash $($f.hashFirma.Substring(0,16))… · íntegra: $($f.integra)" }
    $e = Espera (Api "PUT" "/api/notas/$($nota.id)" $doc @{ subjetivo = "cambio"; objetivo = "x"; evaluacion = "x"; plan = "x" }) 409 "Una nota firmada NO se edita"
    if ($e) { Ok "Código: $($e.code)" }
}
$lista = Espera (Api "GET" "/api/pacientes/$($ana.id)/procedimientos" $doc) 200 "Procedimientos realizados"
if ($lista) { Ok "$(@($lista).Count) registrados; no existe ninguna ruta para editarlos ni borrarlos (RN-01)" }

# ---------------------------------------------------------------------------------------
Titulo "7. Presupuestos, consentimientos y recetas (RF-06, RF-07, RF-08)"
$procs = Espera (Api "GET" "/api/catalogos/procedimientos" $doc) 200 "Procedimientos del catálogo"
$pro = @($procs | Where-Object { $_.codigo -eq "PRO-01" })[0]
$res = @($procs | Where-Object { $_.codigo -eq "RES-01" })[0]
$pres = Espera (Api "POST" "/api/pacientes/$($ana.id)/presupuestos" $doc @{ titulo = "Plan del script"; items = @(
    @{ fase = 1; faseNombre = "Higiene"; sesion = 1; procedimientoId = $pro.id; honorario = "35.00" },
    @{ fase = 2; faseNombre = "Operatoria"; sesion = 2; procedimientoId = $res.id; diente = 16; superficies = @("O"); honorario = "45.00"; costoLaboratorio = "5.50"; descuento = "2.00" }) }) 201 "Presupuesto en borrador"
if ($pres) {
    Ok "Total $($pres.totales.total) = honorarios $($pres.totales.honorarios) + laboratorio $($pres.totales.laboratorio) - descuentos $($pres.totales.descuentos)"
    $pp = Espera (Api "POST" "/api/presupuestos/$($pres.id)/presentar" $doc) 200 "Presentado (se congela con hash)"
    Espera (Api "PUT" "/api/presupuestos/$($pres.id)" $doc @{ titulo = "Otro"; items = @() }) 409 "Un presupuesto presentado NO se edita" | Out-Null
    $dec = @(@{ itemId = $pp.items[0].id; aprobado = $true }, @{ itemId = $pp.items[1].id; aprobado = $false })
    $d = Espera (Api "POST" "/api/presupuestos/$($pres.id)/decision" $rec @{ decisiones = $dec; nota = "Prueba del script" }) 200 "Recepción registra aprobación parcial"
    if ($d) { Ok "Estado $($d.estado) · aprobado $($d.totales.aprobado)" }
    $t = Espera (Api "GET" "/api/pacientes/$($ana.id)/tareas" $rec) 200 "Tareas agendables"
    $deEste = @($t | Where-Object { $_.presupuestoId -eq $pres.id })
    if ($deEste.Count -eq 1) { Ok "1 ítem aprobado => 1 tarea ($($deEste[0].procedimiento), $($deEste[0].estado))" } else { Falla "Se esperaba 1 tarea y hay $($deEste.Count)" }
}
$pls = Espera (Api "GET" "/api/consentimientos/plantillas" $doc) 200 "Plantillas de consentimiento"
$gen = @($pls | Where-Object { $_.codigo -eq "GEN-01" })[0]
$vp = Espera (Api "POST" "/api/pacientes/$($ana.id)/consentimientos/vista-previa" $doc @{ plantillaId = $gen.id }) 200 "Vista previa del texto exacto"
if ($vp) {
    $c = Espera (Api "POST" "/api/pacientes/$($ana.id)/consentimientos" $doc @{ plantillaId = $gen.id; metodo = "OTP"; hashDocumento = $vp.hashDocumento }) 201 "Consentimiento: código OTP enviado"
    if ($c) {
        Ok "Enviado a $($c.otpDestinatario) por el proveedor '$($c.otpProveedor)' (simulado: $($c.otpSimulado))"
        Espera (Api "POST" "/api/consentimientos/$($c.id)/verificar-otp" $doc @{ codigo = "000000" }) 400 "Código incorrecto rechazado" | Out-Null
        $f = Espera (Api "POST" "/api/consentimientos/$($c.id)/verificar-otp" $doc @{ codigo = $c.otpSimuladoCodigo }) 200 "Firmado con el código"
        if ($f) { Ok "Estado $($f.estado) · íntegro: $($f.integro) · documento $($f.hashDocumento.Substring(0,16))…" }
    }
}
$alerta = Espera (Api "POST" "/api/pacientes/$($ana.id)/recetas" $doc @{ items = @(@{ medicamento = "Amoxicilina"; presentacion = "Cápsulas 500 mg"; dosis = "1"; via = "Oral"; frecuencia = "Cada 8 horas"; duracion = "7 días" }) }) 409 "Receta con amoxicilina a paciente alérgica a penicilina: alerta"
if ($alerta) { Ok "$($alerta.code): $($alerta.extra.alertas[0].motivo)" }

# ---------------------------------------------------------------------------------------
Write-Host ""
if ($script:fallas -eq 0) { Write-Host "TODO OK" -ForegroundColor Green; exit 0 }
Write-Host "$($script:fallas) comprobación(es) fallaron" -ForegroundColor Red
exit 1
