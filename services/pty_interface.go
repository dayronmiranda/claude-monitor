package services

import (
	"io"
	"os/exec"
)

// PTY representa una interfaz abstracta para pseudoterminales
// Permite implementaciones diferentes para Unix (creack/pty) y Windows (ConPTY)
type PTY interface {
	// Read lee bytes del PTY
	Read(p []byte) (n int, err error)

	// Write escribe bytes al PTY
	Write(p []byte) (n int, err error)

	// Close cierra el PTY
	Close() error

	// Fd retorna el file descriptor (Unix) o handle (Windows)
	Fd() uintptr

	// Resize redimensiona el PTY
	Resize(cols, rows uint16) error
}

// PTYStarter es la función que inicia un comando con PTY
// Se implementa de forma diferente en Unix y Windows
type PTYStarter interface {
	// Start inicia un comando con un PTY adjunto
	Start(cmd *exec.Cmd) (PTY, error)
}

// ProcessSignaler maneja señales de proceso de forma cross-platform
type ProcessSignaler interface {
	// Terminate envía señal de terminación (SIGTERM en Unix, TerminateProcess en Windows)
	Terminate(cmd *exec.Cmd) error

	// Kill mata el proceso forzosamente (SIGKILL en Unix, TerminateProcess en Windows)
	Kill(cmd *exec.Cmd) error
}

// PTYWrapper envuelve un PTY con métodos adicionales
type PTYWrapper struct {
	pty     PTY
	cmd     *exec.Cmd
	closer  io.Closer
}

// NewPTYWrapper crea un nuevo wrapper
func NewPTYWrapper(pty PTY, cmd *exec.Cmd, closer io.Closer) *PTYWrapper {
	return &PTYWrapper{
		pty:    pty,
		cmd:    cmd,
		closer: closer,
	}
}

// Read lee del PTY
func (w *PTYWrapper) Read(p []byte) (n int, err error) {
	return w.pty.Read(p)
}

// Write escribe al PTY
func (w *PTYWrapper) Write(p []byte) (n int, err error) {
	return w.pty.Write(p)
}

// Close cierra el PTY
func (w *PTYWrapper) Close() error {
	if w.closer != nil {
		return w.closer.Close()
	}
	return w.pty.Close()
}

// Fd retorna el file descriptor
func (w *PTYWrapper) Fd() uintptr {
	return w.pty.Fd()
}

// Resize redimensiona el PTY
func (w *PTYWrapper) Resize(cols, rows uint16) error {
	return w.pty.Resize(cols, rows)
}

// Cmd retorna el comando asociado
func (w *PTYWrapper) Cmd() *exec.Cmd {
	return w.cmd
}
