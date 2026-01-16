//go:build windows
// +build windows

package services

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/UserExistsError/conpty"
)

// WindowsPTY implementa PTY para Windows usando ConPTY
type WindowsPTY struct {
	cpty   *conpty.ConPty
	reader io.Reader
	writer io.Writer
}

// Read lee bytes del PTY
func (p *WindowsPTY) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

// Write escribe bytes al PTY
func (p *WindowsPTY) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

// Close cierra el PTY
func (p *WindowsPTY) Close() error {
	return p.cpty.Close()
}

// Fd retorna 0 en Windows (no hay file descriptor tradicional)
func (p *WindowsPTY) Fd() uintptr {
	return 0
}

// Resize redimensiona el PTY
func (p *WindowsPTY) Resize(cols, rows uint16) error {
	return p.cpty.Resize(int(cols), int(rows))
}

// WindowsPTYStarter implementa PTYStarter para Windows
type WindowsPTYStarter struct{}

// Start inicia un comando con ConPTY en Windows
func (s *WindowsPTYStarter) Start(cmd *exec.Cmd) (PTY, error) {
	// Construir la línea de comando completa
	cmdLine := cmd.Path
	if len(cmd.Args) > 1 {
		cmdLine = strings.Join(cmd.Args, " ")
	}

	// Configurar el directorio de trabajo
	workDir := cmd.Dir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			workDir = GetHomeDir()
		}
	}

	// Crear ConPTY con tamaño inicial de 80x24
	cpty, err := conpty.Start(cmdLine, conpty.ConPtyDimensions(80, 24), conpty.ConPtyWorkDir(workDir))
	if err != nil {
		return nil, fmt.Errorf("error creando ConPTY: %w", err)
	}

	return &WindowsPTY{
		cpty:   cpty,
		reader: cpty,
		writer: cpty,
	}, nil
}

// WindowsProcessSignaler implementa ProcessSignaler para Windows
type WindowsProcessSignaler struct{}

// Terminate envía señal de terminación al proceso en Windows
func (s *WindowsProcessSignaler) Terminate(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		// En Windows, enviamos CTRL_BREAK_EVENT o terminamos el proceso
		// Primero intentamos terminar de forma graceful
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

// Kill mata el proceso forzosamente en Windows
func (s *WindowsProcessSignaler) Kill(cmd *exec.Cmd) error {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}

// GetDefaultShell retorna el shell por defecto en Windows
func GetDefaultShell() string {
	// Preferir PowerShell si está disponible
	pwsh := os.Getenv("COMSPEC")
	if pwsh == "" {
		// Buscar cmd.exe
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		pwsh = filepath.Join(systemRoot, "System32", "cmd.exe")
	}

	// Verificar si PowerShell está disponible
	pwshPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(pwshPath); err == nil {
		return pwshPath
	}

	// Verificar PowerShell Core
	pwshCorePath := `C:\Program Files\PowerShell\7\pwsh.exe`
	if _, err := os.Stat(pwshCorePath); err == nil {
		return pwshCorePath
	}

	return pwsh
}

// GetShellArgs retorna los argumentos para iniciar un shell interactivo en Windows
func GetShellArgs() []string {
	shell := GetDefaultShell()
	if strings.Contains(strings.ToLower(shell), "powershell") || strings.Contains(strings.ToLower(shell), "pwsh") {
		return []string{"-NoLogo", "-NoExit"}
	}
	// cmd.exe
	return []string{}
}

// GetShellExecArgs retorna los argumentos para ejecutar un comando en shell
func GetShellExecArgs(command string) []string {
	shell := GetDefaultShell()
	if strings.Contains(strings.ToLower(shell), "powershell") || strings.Contains(strings.ToLower(shell), "pwsh") {
		return []string{"-Command", command}
	}
	// cmd.exe
	return []string{"/C", command}
}

// NewPTYStarter crea un nuevo PTYStarter para Windows
func NewPTYStarter() PTYStarter {
	return &WindowsPTYStarter{}
}

// NewProcessSignaler crea un nuevo ProcessSignaler para Windows
func NewProcessSignaler() ProcessSignaler {
	return &WindowsProcessSignaler{}
}

// GetHomeDir retorna el directorio home del usuario en Windows
func GetHomeDir() string {
	// USERPROFILE es el estándar en Windows
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	}
	if home == "" {
		home = `C:\Users\Default`
	}
	return home
}

// GetClaudeConfigDir retorna el directorio de configuración de Claude en Windows
func GetClaudeConfigDir() string {
	claudeDir := os.Getenv("CLAUDE_DIR")
	if claudeDir == "" {
		// En Windows, Claude usa %USERPROFILE%\.claude
		claudeDir = filepath.Join(GetHomeDir(), ".claude")
	}
	return claudeDir
}
