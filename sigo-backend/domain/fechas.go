package domain

import "time"

const MayoriaEdad = 18

// Edad cumplida en años.
func Edad(nacimiento, hoy time.Time) int {
	e := hoy.Year() - nacimiento.Year()
	if hoy.Month() < nacimiento.Month() || (hoy.Month() == nacimiento.Month() && hoy.Day() < nacimiento.Day()) {
		e--
	}
	return e
}

// DiaLocal devuelve el inicio y fin (UTC) del día local que contiene `t` en la zona de la clínica.
func DiaLocal(t time.Time, zona *time.Location) (time.Time, time.Time) {
	l := t.In(zona)
	ini := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, zona)
	return ini.UTC(), ini.AddDate(0, 0, 1).UTC()
}
