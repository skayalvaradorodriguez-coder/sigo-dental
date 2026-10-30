package controllers

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/middleware"
	"sigo-backend/models"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
	"sigo-backend/validacion"
)

var (
	errCredenciales = models.NewAPIError(401, "CREDENCIALES_INVALIDAS", "Correo o contraseña incorrectos.")
	errCodigoMFA    = models.NewAPIError(400, "CODIGO_MFA_INVALIDO", "El código no es válido.")
)

// emitirSesion crea la sesión de servidor y su JWT (el `jti` une ambos).
func (h *Controllers) emitirSesion(c *fiber.Ctx, u *models.Usuario, mfa bool) (string, *models.Sesion, error) {
	jti := seguridad.TokenAleatorio(24)
	expira := time.Now().Add(h.Cfg.SesionMaxima)
	if err := h.Repo.CrearSesion(c.UserContext(), jti, u.ID, mfa, c.IP(), c.Get(fiber.HeaderUserAgent), expira); err != nil {
		return "", nil, err
	}
	tok, err := seguridad.GenerarToken(h.Cfg.JWTSecret, u.ID, u.Email, u.Rol, jti, expira)
	ahora := time.Now()
	return tok, &models.Sesion{JTI: jti, UsuarioID: u.ID, MFAVerificado: mfa, CreadaEn: ahora, UltimaActividad: ahora, ExpiraEn: expira}, err
}

func (h *Controllers) perfil(c *fiber.Ctx, u *models.Usuario, s *models.Sesion) fiber.Map {
	paso := middleware.SiguientePaso(u, s)
	var siguiente any
	if paso != "" {
		siguiente = paso
	}
	return fiber.Map{
		"usuario": fiber.Map{
			"id": u.ID, "email": u.Email, "nombre": u.Nombre, "rol": u.Rol, "etiquetaRol": permisos.EtiquetaRol[u.Rol],
			"registroProfesional": u.RegistroProfesional, "mfaActivo": u.MFAActivo, "ultimoAcceso": u.UltimoAcceso,
		},
		"permisos": permisos.PorRol[u.Rol],
		"sesion": fiber.Map{"mfaVerificado": s.MFAVerificado, "expiraEn": s.ExpiraEn,
			"minutosInactividad": h.Ajuste(c.UserContext(), repository.AjusteInactividad)},
		"siguientePaso": siguiente,
	}
}

type credenciales struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login — POST /api/login (primer factor).
func (h *Controllers) Login(c *fiber.Ctx) error {
	var in credenciales
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	errs.Email("email", in.Email, true)
	errs.Requerido("password", in.Password, "Ingrese su contraseña.")
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	ctx := c.UserContext()
	u, err := h.Repo.UsuarioPorEmail(ctx, in.Email)
	if err != nil || !u.Activo {
		seguridad.CompararFicticio(in.Password) // tiempo constante: no revela si el correo existe
		motivo := "USUARIO_DESCONOCIDO"
		uid := ""
		if err == nil {
			motivo, uid = "USUARIO_INACTIVO", u.ID
		}
		h.Auditar(c, repository.Evento{Accion: "LOGIN_FALLIDO", Entidad: "auth", UsuarioID: uid, Detalle: map[string]any{"email": in.Email, "motivo": motivo}})
		return responder(c, errCredenciales)
	}
	if u.BloqueadoHasta != nil && u.BloqueadoHasta.After(time.Now()) {
		h.Auditar(c, repository.Evento{Accion: "CUENTA_BLOQUEADA", Entidad: "auth", UsuarioID: u.ID})
		return responder(c, models.NewAPIError(423, "CUENTA_BLOQUEADA", "Cuenta bloqueada temporalmente por intentos fallidos. Intente más tarde.").Con("bloqueadoHasta", u.BloqueadoHasta))
	}
	if !seguridad.VerificarPassword(u.PasswordHash, in.Password) {
		max := h.Ajuste(ctx, repository.AjusteMaxIntentos)
		min := h.Ajuste(ctx, repository.AjusteMinBloqueo)
		hasta, intentos, err := h.Repo.RegistrarFalloLogin(ctx, u.ID, max, time.Duration(min)*time.Minute)
		if err != nil {
			return h.errorInterno(c, err)
		}
		if hasta != nil {
			h.Auditar(c, repository.Evento{Accion: "CUENTA_BLOQUEADA", Entidad: "auth", UsuarioID: u.ID, Detalle: map[string]any{"bloqueadoHasta": hasta.UTC().Format(time.RFC3339)}})
			return responder(c, models.NewAPIError(423, "CUENTA_BLOQUEADA", "Cuenta bloqueada temporalmente por intentos fallidos. Intente más tarde.").Con("bloqueadoHasta", hasta))
		}
		h.Auditar(c, repository.Evento{Accion: "LOGIN_FALLIDO", Entidad: "auth", UsuarioID: u.ID, Detalle: map[string]any{"motivo": "PASSWORD_INCORRECTO", "intentos": intentos}})
		return responder(c, errCredenciales)
	}
	if err := h.Repo.RegistrarLoginExitoso(ctx, u.ID); err != nil {
		return h.errorInterno(c, err)
	}
	tok, s, err := h.emitirSesion(c, u, false)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "LOGIN_EXITOSO", Entidad: "auth", UsuarioID: u.ID})
	return c.JSON(fiber.Map{"token": tok, "email": u.Email, "rol": u.Rol, "nombre": u.Nombre, "siguientePaso": h.perfil(c, u, s)["siguientePaso"]})
}

type codigoMFA struct {
	Codigo string `json:"codigo"`
}

// VerificarMFA — POST /api/mfa/verificar. Emite un token NUEVO y revoca el anterior
// (evita la fijación de sesión: un token capturado antes del MFA deja de servir).
func (h *Controllers) VerificarMFA(c *fiber.Ctx) error {
	var in codigoMFA
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	a := middleware.Actual(c)
	u := a.Usuario
	if !u.MFAActivo || u.MFASecreto == nil {
		return malaPeticion(c, "MFA_NO_CONFIGURADO", "El usuario no tiene la verificación en dos pasos configurada.")
	}
	paso, ok := seguridad.VerificarTOTP(*u.MFASecreto, in.Codigo, time.Now(), u.MFAUltimoPaso)
	if !ok {
		h.Auditar(c, repository.Evento{Accion: "MFA_FALLIDO", Entidad: "auth"})
		return responder(c, errCodigoMFA)
	}
	ctx := c.UserContext()
	if err := h.Repo.GuardarPasoMFA(ctx, u.ID, paso); err != nil {
		return h.errorInterno(c, err)
	}
	_ = h.Repo.RevocarSesion(ctx, a.Sesion.JTI, "ROTADA")
	tok, s, err := h.emitirSesion(c, u, true)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "MFA_VERIFICADO", Entidad: "auth"})
	r := h.perfil(c, u, s)
	r["token"] = tok
	return c.JSON(r)
}

// ConfigurarMFA — POST /api/mfa/configurar: genera el secreto (la app lo muestra como QR).
func (h *Controllers) ConfigurarMFA(c *fiber.Ctx) error {
	u := middleware.Actual(c).Usuario
	if u.MFAActivo {
		return responder(c, models.NewAPIError(409, "MFA_YA_ACTIVO", "La verificación en dos pasos ya está activa."))
	}
	if u.DebeCambiarPassword {
		return responder(c, models.NewAPIError(403, "CAMBIO_PASSWORD_REQUERIDO", "Debe cambiar su contraseña antes de continuar."))
	}
	secreto := seguridad.GenerarSecretoTOTP()
	if err := h.Repo.GuardarSecretoMFA(c.UserContext(), u.ID, secreto); err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(fiber.Map{"secreto": secreto, "otpauthUrl": seguridad.OtpauthURL(secreto, u.Email, h.Cfg.MFAEmisor)})
}

// ActivarMFA — POST /api/mfa/activar: confirma el enrolamiento con un primer código válido.
func (h *Controllers) ActivarMFA(c *fiber.Ctx) error {
	var in codigoMFA
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	a := middleware.Actual(c)
	ctx := c.UserContext()
	u, err := h.Repo.UsuarioPorID(ctx, a.Usuario.ID)
	if err != nil {
		return h.errorInterno(c, err)
	}
	if u.MFAActivo {
		return responder(c, models.NewAPIError(409, "MFA_YA_ACTIVO", "La verificación en dos pasos ya está activa."))
	}
	if u.MFASecreto == nil {
		return malaPeticion(c, "MFA_NO_INICIADO", "Primero inicie la configuración.")
	}
	paso, ok := seguridad.VerificarTOTP(*u.MFASecreto, in.Codigo, time.Now(), nil)
	if !ok {
		h.Auditar(c, repository.Evento{Accion: "MFA_FALLIDO", Entidad: "auth", Detalle: map[string]any{"enrolamiento": true}})
		return responder(c, errCodigoMFA)
	}
	if err := h.Repo.ActivarMFA(ctx, u.ID, paso); err != nil {
		return h.errorInterno(c, err)
	}
	u.MFAActivo = true
	_ = h.Repo.RevocarSesion(ctx, a.Sesion.JTI, "ROTADA")
	tok, s, err := h.emitirSesion(c, u, true)
	if err != nil {
		return h.errorInterno(c, err)
	}
	h.Auditar(c, repository.Evento{Accion: "MFA_ACTIVADO", Entidad: "auth"})
	r := h.perfil(c, u, s)
	r["token"] = tok
	return c.JSON(r)
}

type cambioPassword struct {
	Actual string `json:"actual"`
	Nueva  string `json:"nueva"`
}

// CambiarPassword — POST /api/password. Cierra las demás sesiones del usuario.
func (h *Controllers) CambiarPassword(c *fiber.Ctx) error {
	var in cambioPassword
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	a := middleware.Actual(c)
	u := a.Usuario
	if u.MFAActivo && !a.Sesion.MFAVerificado {
		return responder(c, models.NewAPIError(403, "MFA_REQUERIDO", "Debe completar la verificación en dos pasos."))
	}
	if !seguridad.VerificarPassword(u.PasswordHash, in.Actual) {
		return malaPeticion(c, "PASSWORD_ACTUAL_INCORRECTO", "La contraseña actual no es correcta.")
	}
	errs := seguridad.ErroresPolitica(in.Nueva, u.Email)
	if in.Actual == in.Nueva {
		errs = append(errs, "La nueva contraseña debe ser distinta de la actual.")
	}
	if len(errs) > 0 {
		return responder(c, models.NewAPIError(400, "PASSWORD_DEBIL", joinMsgs(errs)).Con("errores", errs))
	}
	hash, err := seguridad.HashPassword(in.Nueva)
	if err != nil {
		return h.errorInterno(c, err)
	}
	ctx := c.UserContext()
	if err := h.Repo.CambiarPassword(ctx, u.ID, hash, false); err != nil {
		return h.errorInterno(c, err)
	}
	_ = h.Repo.RevocarSesionesUsuario(ctx, u.ID, a.Sesion.JTI, "PASSWORD_CAMBIADO")
	h.Auditar(c, repository.Evento{Accion: "PASSWORD_CAMBIADO", Entidad: "auth"})
	return c.SendStatus(204)
}

func joinMsgs(m []string) string {
	out := ""
	for i, s := range m {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

// Logout — POST /api/logout: revoca la sesión (el token deja de servir al instante).
func (h *Controllers) Logout(c *fiber.Ctx) error {
	a := middleware.Actual(c)
	_ = h.Repo.RevocarSesion(c.UserContext(), a.Sesion.JTI, "LOGOUT")
	h.Auditar(c, repository.Evento{Accion: "LOGOUT", Entidad: "auth"})
	return c.SendStatus(204)
}

// Me — GET /api/me: usuario, permisos, sesión y paso pendiente.
func (h *Controllers) Me(c *fiber.Ctx) error {
	a := middleware.Actual(c)
	return c.JSON(h.perfil(c, a.Usuario, a.Sesion))
}

// Heartbeat — POST /api/heartbeat: el frontend lo llama (máx. 1/min) mientras hay actividad
// de teclado/ratón, para que escribir una nota larga no cuente como inactividad.
func (h *Controllers) Heartbeat(c *fiber.Ctx) error { return c.SendStatus(204) }
