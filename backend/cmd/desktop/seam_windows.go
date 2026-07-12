//go:build windows

// Package desktop marks the Windows-only host boundary. Child 03 deliberately
// exposes no runnable product entry point; later desktop hosts must call the
// shared application.Runtime through internal/desktop/bootstrap.
package desktop
