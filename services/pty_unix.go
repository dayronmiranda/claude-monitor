//go:build !windows
// +build !windows

package services

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
)

// UnixPTY implementa PTY para sistemas Unix (Linux, macOS)
type UnixPTY struct {
	file *os.File
}

// Read lee bytes del PTY
func (p *UnixPTY) Read(b []byte) (n int, err error) {
	return p.file.Read(b)
}

// Write escribe bytes al PTY
func (p *UnixPTY) Write(b []byte) (n int, err error) {
	return p.file.Write(b)
}

// Close cierra el PTY
func (p *UnixPTY) Close() error {
	return p.file.Close()
}

// Fd retorna el file descriptor
func (p *UnixPTY) Fd() uintptr {
	return p.file.Fd()
}

// Resize redimensiona el PTY usando ioctl
func (p *UnixPTY) Resize(cols, rows uint16) error {
	ws := struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{Row: rows, Col: cols}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		p.file.Fd(),
		syscall.TIOCSWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// UnixPTYStarter implementa PTYStarter para Unix
type UnixPTYStarter struct{}

// Start inicia un comando con PTY en Unix
func (s *UnixPTYStarter) Start(cmd *exec.Cmd) (PTY, error) {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &UnixPTY{file: ptmx}, nil
}

// UnixProcessSignaler implementa ProcessSignaler para Unix
type UnixProcessSignaler struct{}

// Terminate envía SIGTERM al proceso
func (s *UnixProcessSignaler) Terminate(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

// Kill envía SIGKILL al proceso
func (s *UnixProcessSignaler) Kill(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// GetDefaultShell retorna el shell por defecto en Unix
func GetDefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}

// GetShellArgs retorna los argumentos para iniciar un shell interactivo
func GetShellArgs() []string {
	return []string{"-l"}
}

// GetShellExecArgs retorna los argumentos para ejecutar un comando en shell
func GetShellExecArgs(command string) []string {
	return []string{"-c", command}
}

// NewPTYStarter crea un nuevo PTYStarter para la plataforma actual
func NewPTYStarter() PTYStarter {
	return &UnixPTYStarter{}
}

// NewProcessSignaler crea un nuevo ProcessSignaler para la plataforma actual
func NewProcessSignaler() ProcessSignaler {
	return &UnixProcessSignaler{}
}

// GetHomeDir retorna el directorio home del usuario
func GetHomeDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return home
}

// GetClaudeConfigDir retorna el directorio de configuración de Claude
func GetClaudeConfigDir() string {
	claudeDir := os.Getenv("CLAUDE_DIR")
	if claudeDir == "" {
		claudeDir = GetHomeDir() + "/.claude"
	}
	return claudeDir
}
