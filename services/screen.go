package services

import (
	"strings"
	"sync"

	"github.com/Azure/go-ansiterm"
)

// Colores ANSI
const (
	ColorBlack   = 0
	ColorRed     = 1
	ColorGreen   = 2
	ColorYellow  = 3
	ColorBlue    = 4
	ColorMagenta = 5
	ColorCyan    = 6
	ColorWhite   = 7
	ColorDefault = 9
)

// Cell representa una celda de la pantalla con su carácter y atributos
type Cell struct {
	Char rune
	FG   int // Foreground color (0-7, 9=default)
	BG   int // Background color (0-7, 9=default)
	Bold bool
	Dim  bool
}

// ScreenHandler implementa AnsiEventHandler de go-ansiterm
// Mantiene el estado completo de una pantalla de terminal virtual
type ScreenHandler struct {
	mu sync.RWMutex

	// Dimensiones
	width  int
	height int

	// Buffer de pantalla (main y alternate)
	buffer    [][]Cell
	altBuffer [][]Cell
	inAltMode bool

	// Cursor
	cursorX int
	cursorY int

	// Atributos actuales
	currentFG   int
	currentBG   int
	currentBold bool
	currentDim  bool

	// Scroll region
	scrollTop    int
	scrollBottom int

	// Historia (scrollback)
	history       [][]Cell
	maxHistory    int
	historyOffset int
}

// NewScreenHandler crea un nuevo handler con las dimensiones especificadas
func NewScreenHandler(width, height int) *ScreenHandler {
	h := &ScreenHandler{
		width:        width,
		height:       height,
		currentFG:    ColorDefault,
		currentBG:    ColorDefault,
		scrollTop:    0,
		scrollBottom: height - 1,
		maxHistory:   1000,
		history:      make([][]Cell, 0),
	}
	h.buffer = h.makeBuffer(width, height)
	h.altBuffer = h.makeBuffer(width, height)
	return h
}

// makeBuffer crea un buffer vacío
func (h *ScreenHandler) makeBuffer(width, height int) [][]Cell {
	buf := make([][]Cell, height)
	for i := range buf {
		buf[i] = make([]Cell, width)
		for j := range buf[i] {
			buf[i][j] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}
	return buf
}

// currentBuffer retorna el buffer activo
func (h *ScreenHandler) currentBuffer() [][]Cell {
	if h.inAltMode {
		return h.altBuffer
	}
	return h.buffer
}

// ============================================================================
// Implementación de AnsiEventHandler
// ============================================================================

// Print imprime un carácter en la posición actual del cursor
func (h *ScreenHandler) Print(b byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cursorX >= h.width {
		h.cursorX = 0
		h.cursorY++
		if h.cursorY > h.scrollBottom {
			h.scrollUp(1)
			h.cursorY = h.scrollBottom
		}
	}

	buf := h.currentBuffer()
	if h.cursorY >= 0 && h.cursorY < h.height && h.cursorX >= 0 && h.cursorX < h.width {
		buf[h.cursorY][h.cursorX] = Cell{
			Char: rune(b),
			FG:   h.currentFG,
			BG:   h.currentBG,
			Bold: h.currentBold,
			Dim:  h.currentDim,
		}
	}
	h.cursorX++

	return nil
}

// Execute ejecuta un carácter de control (C0/C1)
func (h *ScreenHandler) Execute(b byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch b {
	case 0x07: // BEL - Bell
		// Ignorar beep
	case 0x08: // BS - Backspace
		if h.cursorX > 0 {
			h.cursorX--
		}
	case 0x09: // HT - Tab
		h.cursorX = ((h.cursorX / 8) + 1) * 8
		if h.cursorX >= h.width {
			h.cursorX = h.width - 1
		}
	case 0x0A, 0x0B, 0x0C: // LF, VT, FF - Line Feed
		h.cursorY++
		if h.cursorY > h.scrollBottom {
			h.scrollUp(1)
			h.cursorY = h.scrollBottom
		}
	case 0x0D: // CR - Carriage Return
		h.cursorX = 0
	}

	return nil
}

// CUU - Cursor Up
func (h *ScreenHandler) CUU(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY -= count
	if h.cursorY < h.scrollTop {
		h.cursorY = h.scrollTop
	}
	return nil
}

// CUD - Cursor Down
func (h *ScreenHandler) CUD(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY += count
	if h.cursorY > h.scrollBottom {
		h.cursorY = h.scrollBottom
	}
	return nil
}

// CUF - Cursor Forward
func (h *ScreenHandler) CUF(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorX += count
	if h.cursorX >= h.width {
		h.cursorX = h.width - 1
	}
	return nil
}

// CUB - Cursor Backward
func (h *ScreenHandler) CUB(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorX -= count
	if h.cursorX < 0 {
		h.cursorX = 0
	}
	return nil
}

// CNL - Cursor Next Line
func (h *ScreenHandler) CNL(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorX = 0
	h.cursorY += count
	if h.cursorY > h.scrollBottom {
		h.cursorY = h.scrollBottom
	}
	return nil
}

// CPL - Cursor Previous Line
func (h *ScreenHandler) CPL(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorX = 0
	h.cursorY -= count
	if h.cursorY < h.scrollTop {
		h.cursorY = h.scrollTop
	}
	return nil
}

// CHA - Cursor Horizontal Absolute
func (h *ScreenHandler) CHA(col int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorX = col - 1 // 1-based to 0-based
	if h.cursorX < 0 {
		h.cursorX = 0
	}
	if h.cursorX >= h.width {
		h.cursorX = h.width - 1
	}
	return nil
}

// VPA - Vertical Position Absolute
func (h *ScreenHandler) VPA(row int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY = row - 1 // 1-based to 0-based
	if h.cursorY < 0 {
		h.cursorY = 0
	}
	if h.cursorY >= h.height {
		h.cursorY = h.height - 1
	}
	return nil
}

// CUP - Cursor Position
func (h *ScreenHandler) CUP(row, col int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY = row - 1
	h.cursorX = col - 1

	if h.cursorX < 0 {
		h.cursorX = 0
	}
	if h.cursorX >= h.width {
		h.cursorX = h.width - 1
	}
	if h.cursorY < 0 {
		h.cursorY = 0
	}
	if h.cursorY >= h.height {
		h.cursorY = h.height - 1
	}
	return nil
}

// HVP - Horizontal and Vertical Position (same as CUP)
func (h *ScreenHandler) HVP(row, col int) error {
	return h.CUP(row, col)
}

// DECTCEM - Text Cursor Enable Mode
func (h *ScreenHandler) DECTCEM(visible bool) error {
	// Cursor visibility - we track state but don't need to do anything
	return nil
}

// DECOM - Origin Mode
func (h *ScreenHandler) DECOM(enable bool) error {
	// Origin mode - affects cursor positioning relative to scroll region
	return nil
}

// DECCOLM - 132 Column Mode
func (h *ScreenHandler) DECCOLM(use132 bool) error {
	// Column mode switching - not commonly used
	return nil
}

// ED - Erase in Display
func (h *ScreenHandler) ED(mode int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	switch mode {
	case 0: // Erase from cursor to end of screen
		// Clear rest of current line
		for x := h.cursorX; x < h.width; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
		// Clear all lines below
		for y := h.cursorY + 1; y < h.height; y++ {
			for x := 0; x < h.width; x++ {
				buf[y][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
			}
		}
	case 1: // Erase from start of screen to cursor
		// Clear all lines above
		for y := 0; y < h.cursorY; y++ {
			for x := 0; x < h.width; x++ {
				buf[y][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
			}
		}
		// Clear current line up to cursor
		for x := 0; x <= h.cursorX; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	case 2: // Erase entire screen
		for y := 0; y < h.height; y++ {
			for x := 0; x < h.width; x++ {
				buf[y][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
			}
		}
	}

	return nil
}

// EL - Erase in Line
func (h *ScreenHandler) EL(mode int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	switch mode {
	case 0: // Erase from cursor to end of line
		for x := h.cursorX; x < h.width; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	case 1: // Erase from start of line to cursor
		for x := 0; x <= h.cursorX; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	case 2: // Erase entire line
		for x := 0; x < h.width; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}

	return nil
}

// IL - Insert Line
func (h *ScreenHandler) IL(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Move lines down
		for y := h.scrollBottom; y > h.cursorY; y-- {
			copy(buf[y], buf[y-1])
		}
		// Clear current line
		for x := 0; x < h.width; x++ {
			buf[h.cursorY][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}

	return nil
}

// DL - Delete Line
func (h *ScreenHandler) DL(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Move lines up
		for y := h.cursorY; y < h.scrollBottom; y++ {
			copy(buf[y], buf[y+1])
		}
		// Clear bottom line
		for x := 0; x < h.width; x++ {
			buf[h.scrollBottom][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}

	return nil
}

// ICH - Insert Character
func (h *ScreenHandler) ICH(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Shift characters right
		for x := h.width - 1; x > h.cursorX; x-- {
			buf[h.cursorY][x] = buf[h.cursorY][x-1]
		}
		// Insert blank at cursor
		buf[h.cursorY][h.cursorX] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
	}

	return nil
}

// DCH - Delete Character
func (h *ScreenHandler) DCH(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Shift characters left
		for x := h.cursorX; x < h.width-1; x++ {
			buf[h.cursorY][x] = buf[h.cursorY][x+1]
		}
		// Insert blank at end
		buf[h.cursorY][h.width-1] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
	}

	return nil
}

// SGR - Set Graphics Rendition (colores y estilos)
func (h *ScreenHandler) SGR(params []int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(params) == 0 {
		params = []int{0} // Reset
	}

	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0: // Reset
			h.currentFG = ColorDefault
			h.currentBG = ColorDefault
			h.currentBold = false
			h.currentDim = false
		case p == 1: // Bold
			h.currentBold = true
		case p == 2: // Dim
			h.currentDim = true
		case p == 22: // Normal intensity
			h.currentBold = false
			h.currentDim = false
		case p >= 30 && p <= 37: // Foreground color
			h.currentFG = p - 30
		case p == 39: // Default foreground
			h.currentFG = ColorDefault
		case p >= 40 && p <= 47: // Background color
			h.currentBG = p - 40
		case p == 49: // Default background
			h.currentBG = ColorDefault
		case p >= 90 && p <= 97: // Bright foreground
			h.currentFG = p - 90 + 8
		case p >= 100 && p <= 107: // Bright background
			h.currentBG = p - 100 + 8
		case p == 38: // Extended foreground (256 color or RGB)
			if i+2 < len(params) && params[i+1] == 5 {
				// 256 color mode: 38;5;n
				h.currentFG = params[i+2]
				i += 2
			} else if i+4 < len(params) && params[i+1] == 2 {
				// RGB mode: 38;2;r;g;b - simplify to nearest
				i += 4
			}
		case p == 48: // Extended background
			if i+2 < len(params) && params[i+1] == 5 {
				h.currentBG = params[i+2]
				i += 2
			} else if i+4 < len(params) && params[i+1] == 2 {
				i += 4
			}
		}
	}

	return nil
}

// SU - Scroll Up (Pan Down)
func (h *ScreenHandler) SU(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollUp(count)
	return nil
}

// SD - Scroll Down (Pan Up)
func (h *ScreenHandler) SD(count int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollDown(count)
	return nil
}

// scrollUp mueve líneas hacia arriba (usado internamente)
func (h *ScreenHandler) scrollUp(count int) {
	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Save top line to history (only for main buffer)
		if !h.inAltMode && len(h.history) < h.maxHistory {
			lineCopy := make([]Cell, h.width)
			copy(lineCopy, buf[h.scrollTop])
			h.history = append(h.history, lineCopy)
		}

		// Move lines up
		for y := h.scrollTop; y < h.scrollBottom; y++ {
			copy(buf[y], buf[y+1])
		}
		// Clear bottom line
		for x := 0; x < h.width; x++ {
			buf[h.scrollBottom][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}
}

// scrollDown mueve líneas hacia abajo
func (h *ScreenHandler) scrollDown(count int) {
	buf := h.currentBuffer()

	for i := 0; i < count; i++ {
		// Move lines down
		for y := h.scrollBottom; y > h.scrollTop; y-- {
			copy(buf[y], buf[y-1])
		}
		// Clear top line
		for x := 0; x < h.width; x++ {
			buf[h.scrollTop][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
		}
	}
}

// DA - Device Attributes
func (h *ScreenHandler) DA(attrs []string) error {
	// Device attributes query - we don't need to respond
	return nil
}

// DECSTBM - Set Top and Bottom Margins (scroll region)
func (h *ScreenHandler) DECSTBM(top, bottom int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.scrollTop = top - 1
	h.scrollBottom = bottom - 1

	if h.scrollTop < 0 {
		h.scrollTop = 0
	}
	if h.scrollBottom >= h.height {
		h.scrollBottom = h.height - 1
	}
	if h.scrollTop >= h.scrollBottom {
		h.scrollTop = 0
		h.scrollBottom = h.height - 1
	}

	// Move cursor to home
	h.cursorX = 0
	h.cursorY = h.scrollTop

	return nil
}

// IND - Index (move cursor down, scroll if needed)
func (h *ScreenHandler) IND() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY++
	if h.cursorY > h.scrollBottom {
		h.scrollUp(1)
		h.cursorY = h.scrollBottom
	}
	return nil
}

// RI - Reverse Index (move cursor up, scroll if needed)
func (h *ScreenHandler) RI() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cursorY--
	if h.cursorY < h.scrollTop {
		h.scrollDown(1)
		h.cursorY = h.scrollTop
	}
	return nil
}

// Flush - llamado cuando se debe renderizar
func (h *ScreenHandler) Flush() error {
	// No-op for our implementation
	return nil
}

// ============================================================================
// Métodos adicionales para control de pantalla (no parte de AnsiEventHandler)
// ============================================================================

// SetAlternateMode cambia al alternate screen buffer (usado por vim, htop, etc)
func (h *ScreenHandler) SetAlternateMode(enable bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if enable && !h.inAltMode {
		// Switch to alternate buffer
		h.inAltMode = true
		h.cursorX = 0
		h.cursorY = 0
		// Clear alternate buffer
		for y := 0; y < h.height; y++ {
			for x := 0; x < h.width; x++ {
				h.altBuffer[y][x] = Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
			}
		}
	} else if !enable && h.inAltMode {
		// Switch back to main buffer
		h.inAltMode = false
	}
}

// ============================================================================
// Métodos públicos para obtener estado
// ============================================================================

// String retorna el contenido de la pantalla como texto plano
func (h *ScreenHandler) String() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.currentBuffer()
	var sb strings.Builder

	for y := 0; y < h.height; y++ {
		for x := 0; x < h.width; x++ {
			ch := buf[y][x].Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		if y < h.height-1 {
			sb.WriteRune('\n')
		}
	}

	return sb.String()
}

// GetDisplay retorna las líneas de la pantalla (similar a pyte screen.display)
func (h *ScreenHandler) GetDisplay() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.currentBuffer()
	lines := make([]string, h.height)

	for y := 0; y < h.height; y++ {
		var sb strings.Builder
		for x := 0; x < h.width; x++ {
			ch := buf[y][x].Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		lines[y] = strings.TrimRight(sb.String(), " ")
	}

	return lines
}

// GetCursor retorna la posición actual del cursor
func (h *ScreenHandler) GetCursor() (x, y int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.cursorX, h.cursorY
}

// GetSize retorna las dimensiones
func (h *ScreenHandler) GetSize() (width, height int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.width, h.height
}

// IsInAlternateScreen retorna si está en modo alternativo (vim, htop, etc)
func (h *ScreenHandler) IsInAlternateScreen() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.inAltMode
}

// GetCell retorna información de una celda específica
func (h *ScreenHandler) GetCell(x, y int) Cell {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.currentBuffer()
	if y >= 0 && y < h.height && x >= 0 && x < h.width {
		return buf[y][x]
	}
	return Cell{Char: ' ', FG: ColorDefault, BG: ColorDefault}
}

// GetHistory retorna el historial de scroll
func (h *ScreenHandler) GetHistory() [][]Cell {
	h.mu.RLock()
	defer h.mu.RUnlock()

	historyCopy := make([][]Cell, len(h.history))
	for i, line := range h.history {
		historyCopy[i] = make([]Cell, len(line))
		copy(historyCopy[i], line)
	}
	return historyCopy
}

// GetHistoryLines retorna el historial como strings
func (h *ScreenHandler) GetHistoryLines() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	lines := make([]string, len(h.history))
	for i, row := range h.history {
		var sb strings.Builder
		for _, cell := range row {
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		lines[i] = strings.TrimRight(sb.String(), " ")
	}
	return lines
}

// Resize redimensiona la pantalla
func (h *ScreenHandler) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create new buffers
	newBuffer := h.makeBuffer(width, height)
	newAltBuffer := h.makeBuffer(width, height)

	// Copy content from old buffers
	minHeight := h.height
	if height < minHeight {
		minHeight = height
	}
	minWidth := h.width
	if width < minWidth {
		minWidth = width
	}

	for y := 0; y < minHeight; y++ {
		for x := 0; x < minWidth; x++ {
			newBuffer[y][x] = h.buffer[y][x]
			newAltBuffer[y][x] = h.altBuffer[y][x]
		}
	}

	h.buffer = newBuffer
	h.altBuffer = newAltBuffer
	h.width = width
	h.height = height

	// Adjust cursor
	if h.cursorX >= width {
		h.cursorX = width - 1
	}
	if h.cursorY >= height {
		h.cursorY = height - 1
	}

	// Adjust scroll region
	h.scrollTop = 0
	h.scrollBottom = height - 1
}

// ============================================================================
// ScreenState wrapper que combina ScreenHandler con AnsiParser
// ============================================================================

// ScreenState combina el handler con el parser de go-ansiterm
type ScreenState struct {
	handler *ScreenHandler
	parser  *ansiterm.AnsiParser
	mu      sync.Mutex
}

// NewScreenState crea un nuevo ScreenState
func NewScreenState(width, height int) *ScreenState {
	handler := NewScreenHandler(width, height)
	parser := ansiterm.CreateParser("Ground", handler)

	return &ScreenState{
		handler: handler,
		parser:  parser,
	}
}

// Feed procesa bytes de output del terminal
func (s *ScreenState) Feed(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.parser.Parse(data)
	return err
}

// Snapshot retorna el estado actual de la pantalla como texto
func (s *ScreenState) Snapshot() string {
	return s.handler.String()
}

// GetDisplay retorna las líneas de la pantalla
func (s *ScreenState) GetDisplay() []string {
	return s.handler.GetDisplay()
}

// GetCursor retorna la posición del cursor
func (s *ScreenState) GetCursor() (x, y int) {
	return s.handler.GetCursor()
}

// GetSize retorna dimensiones
func (s *ScreenState) GetSize() (width, height int) {
	return s.handler.GetSize()
}

// IsInAlternateScreen indica si está en alternate screen (vim, htop)
func (s *ScreenState) IsInAlternateScreen() bool {
	return s.handler.IsInAlternateScreen()
}

// GetCell retorna una celda específica
func (s *ScreenState) GetCell(x, y int) Cell {
	return s.handler.GetCell(x, y)
}

// GetHistory retorna historial de scroll
func (s *ScreenState) GetHistoryLines() []string {
	return s.handler.GetHistoryLines()
}

// Resize redimensiona la pantalla
func (s *ScreenState) Resize(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler.Resize(width, height)
}

// SetAlternateMode cambia el modo de pantalla
func (s *ScreenState) SetAlternateMode(enable bool) {
	s.handler.SetAlternateMode(enable)
}
