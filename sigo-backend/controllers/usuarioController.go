package controllers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"sigo-backend/middleware"
	"sigo-backend/permisos"
	"sigo-backend/repository"
	"sigo-backend/seguridad"
	"sigo-backend/validacion"
)

// ListarUsuarios — GET /api/usuarios (nunca devuelve hashes ni secretos MFA).
func (h *Controllers) ListarUsuarios(c *fiber.Ctx) error {
	us, err := h.Repo.ListarUsuarios(c.UserContext())
	if err != nil {
		return h.errorInterno(c, err)
	}
	return c.JSON(us)
}

// Roles — GET /api/usuarios/roles: matriz RBAC para mostrar en la interfaz.
func (h *Controllers) Roles(c *fiber.Ctx) error {
	out := []fiber.Map{}
	for _, r := range permisos.Roles {
		out = append(out, fiber.Map{"rol": r, "etiqueta": permisos.EtiquetaRol[r], "permisos": permisos.PorRol[r]})
	}
	return c.JSON(out)
}

type nuevoUsuarioIn struct {
	Email               string `json:"email"`
	Nombre              string `json:"nombre"`
	Rol                 string `json:"rol"`
	RegistroProfesional string `json:"registroProfesional"`
}

// CrearUsuario — POST /api/usuarios: contraseña temporal (se muestra una sola vez);
// al entrar deberá cambiarla y configurar MFA.
func (h *Controllers) CrearUsuario(c *fiber.Ctx) error {
	var in nuevoUsuarioIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	errs := validacion.Errores{}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	errs.Email("email", in.Email, true)
	errs.Longitud("nombre", in.Nombre, 3, 150, "Ingrese el nombre completo.")
	errs.EnLista("rol", in.Rol, permisos.Roles, "Rol inválido.")
	if in.Rol == permisos.Odontologo && strings.TrimSpace(in.RegistroProfesional) == "" {
		errs.Agregar("registroProfesional", "El registro profesional es obligatorio para odontólogos.")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	temp := seguridad.PasswordTemporal()
	hash, err := seguridad.HashPassword(temp)
	if err != nil {
		return h.errorInterno(c, err)
	}
	u, err := h.Repo.CrearUsuario(c.UserContext(), repository.NuevoUsuario{Email: in.Email, Nombre: strings.TrimSpace(in.Nombre), Rol: in.Rol,
		RegistroProfesional: validacion.Opcional(in.RegistroProfesional), PasswordHash: hash, DebeCambiarPassword: true})
	if err != nil {
		return h.errorRepo(c, err, "", "Ya existe un usuario con ese correo.")
	}
	h.Auditar(c, repository.Evento{Accion: "CREACION", Entidad: "usuario", EntidadID: u.ID, Detalle: map[string]any{"email": u.Email, "rol": u.Rol}})
	return c.Status(201).JSON(fiber.Map{"usuario": u, "passwordTemporal": temp})
}

type cambiosUsuarioIn struct {
	Nombre              *string `json:"nombre"`
	Rol                 *string `json:"rol"`
	RegistroProfesional *string `json:"registroProfesional"`
	Activo              *bool   `json:"activo"`
}

// ActualizarUsuario — PATCH /api/usuarios/:id
func (h *Controllers) ActualizarUsuario(c *fiber.Ctx) error {
	id := c.Params("id")
	if !validacion.EsUUID(id) {
		return malaPeticion(c, "VALIDACION", "Identificador inválido.")
	}
	var in cambiosUsuarioIn
	if err := leerJSON(c, &in); err != nil {
		return err
	}
	ctx := c.UserContext()
	actor := middleware.Actual(c).Usuario
	antes, err := h.Repo.UsuarioPorID(ctx, id)
	if err != nil {
		return h.errorRepo(c, err, "Usuario no encontrado.", "")
	}
	errs := validacion.Errores{}
	if in.Nombre != nil {
		errs.Longitud("nombre", *in.Nombre, 3, 150, "Ingrese el nombre completo.")
	}
	if in.Rol != nil {
		errs.EnLista("rol", *in.Rol, permisos.Roles, "Rol inválido.")
	}
	rol := antes.Rol
	if in.Rol != nil {
		rol = *in.Rol
	}
	reg := antes.RegistroProfesional
	if in.RegistroProfesional != nil {
		reg = validacion.Opcional(*in.RegistroProfesional)
	}
	if rol == permisos.Odontologo && reg == nil {
		errs.Agregar("registroProfesional", "El registro profesional es obligatorio para odontólogos.")
	}
	cambiaRol := in.Rol != nil && *in.Rol != antes.Rol
	desactiva := in.Activo != nil && !*in.Activo
	if id == actor.ID && (cambiaRol || desactiva) {
		errs.Agregar("activo", "No puede desactivarse ni cambiar su propio rol.")
	}
	if !errs.Ok() {
		return errValidacion(c, errs)
	}
	if antes.Rol == permisos.AdminTI && (cambiaRol || desactiva) {
		if n, err := h.Repo.ContarAdminsActivos(ctx, id); err != nil {
			return h.errorInterno(c, err)
		} else if n == 0 {
			return malaPeticion(c, "ULTIMO_ADMIN", "Debe quedar al menos un administrador activo.")
		}
	}
	cambios := repository.CambiosUsuario{Nombre: in.Nombre, Rol: in.Rol, Activo: in.Activo}
	if in.RegistroProfesional != nil {
		cambios.RegistroProfesional = &reg
	}
	despues, err := h.Repo.ActualizarUsuario(ctx, id, cambios)
	if err != nil {
		return h.errorRepo(c, err, "Usuario no encontrado.", "")
	}
	if cambiaRol || desactiva {
		motivo := "ROL_CAMBIADO"
		if desactiva {
			motivo = "USUARIO_DESACTIVADO"
		}
		_ = h.Repo.RevocarSesionesUsuario(ctx, id, "", motivo)
	}
	dif := map[string]any{}
	comparar := func(campo string, a, b any) {
		if fmt.Sprint(a) != fmt.Sprint(b) {
			dif[campo] = map[string]any{"de": a, "a": b}
		}
	}
	comparar("nombre", antes.Nombre, despues.Nombre)
	comparar("rol", antes.Rol, despues.Rol)
	comparar("activo", antes.Activo, despues.Activo)
	comparar("registroProfesional", deref(antes.RegistroProfesional), deref(despues.RegistroProfesional))
	h.Auditar(c, repository.Evento{Accion: "MODIFICACION", Entidad: "usuario", EntidadID: id, Detalle: map[string]any{"cambios": dif}})
	return c.JSON(despues)
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// RestablecerPassword — POST /api/usuarios/:id/restablecer-password
func (h *Controllers) RestablecerPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()
	if _, err := h.Repo.UsuarioPorID(ctx, id); err != nil || !validacion.EsUUID(id) {
		return noEncontrado(c, "Usuario no encontrado.")
	}
	temp := seguridad.PasswordTemporal()
	hash, err := seguridad.HashPassword(temp)
	if err != nil {
		return h.errorInterno(c, err)
	}
	if err := h.Repo.CambiarPassword(ctx, id, hash, true); err != nil {
		return h.errorInterno(c, err)
	}
	_ = h.Repo.RevocarSesionesUsuario(ctx, id, "", "PASSWORD_RESTABLECIDO")
	h.Auditar(c, repository.Evento{Accion: "PASSWORD_RESTABLECIDO", Entidad: "usuario", EntidadID: id})
	return c.JSON(fiber.Map{"passwordTemporal": temp})
}

// RestablecerMFA — POST /api/usuarios/:id/restablecer-mfa (p. ej. perdió el teléfono).
func (h *Controllers) RestablecerMFA(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := c.UserContext()
	if _, err := h.Repo.UsuarioPorID(ctx, id); err != nil || !validacion.EsUUID(id) {
		return noEncontrado(c, "Usuario no encontrado.")
	}
	if err := h.Repo.ReiniciarMFA(ctx, id); err != nil {
		return h.errorInterno(c, err)
	}
	_ = h.Repo.RevocarSesionesUsuario(ctx, id, "", "MFA_RESTABLECIDO")
	h.Auditar(c, repository.Evento{Accion: "MFA_RESTABLECIDO", Entidad: "usuario", EntidadID: id})
	return c.SendStatus(204)
}
