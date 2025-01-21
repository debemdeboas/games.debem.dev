package game

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"golang.org/x/exp/rand"
)

const (
	TICKDURATION = 16 * time.Millisecond
	INITIALSPEED = 8

	UP    = 1
	DOWN  = 2
	LEFT  = 3
	RIGHT = 4

	BOARDWIDTH  = 26
	BOARDHEIGHT = 34

	BUFFEREDDIRECTIONCHANGES = 5
)

type Position struct {
	X, Y int
}

type Model struct {
	Term    string
	Profile string
	Width   int
	Height  int
	Bg      string

	// Styles
	TxtStyle       lipgloss.Style
	QuitStyle      lipgloss.Style
	FoodStyle      lipgloss.Style
	SnakeStyle     lipgloss.Style
	GameBoardStyle lipgloss.Style

	// Game state
	tickCount int
	moveSpeed int
	snake     []Position
	direction int
	dirChan   chan int
	lastDir   int
	food      Position
	score     int
	gameOver  bool
	pause     bool

	// Board
	boardWidth  int
	boardHeight int
	offsetX     int
	offsetY     int
}

type tickMsg time.Time

func NewModel(term string, profile string, width, height int, bg string, styles ...lipgloss.Style) *Model {
	m := &Model{
		Term:        term,
		Profile:     profile,
		Width:       width,
		Height:      height,
		Bg:          bg,
		boardWidth:  BOARDWIDTH,
		boardHeight: BOARDHEIGHT,
		offsetX:     (BOARDWIDTH - width) / 2,
		offsetY:     (BOARDHEIGHT - height) / 2,
	}

	// Apply styles if provided
	if len(styles) >= 5 {
		m.TxtStyle = styles[0]
		m.QuitStyle = styles[1]
		m.FoodStyle = styles[2]
		m.SnakeStyle = styles[3]
		m.GameBoardStyle = styles[4]
	}

	m.RestartGame()
	return m
}

func (m Model) Init() tea.Cmd {
	return m.tick()
}

func (m *Model) RestartGame() {
	initialX := m.boardWidth / 2
	initialY := m.boardHeight / 2

	initialSnake := []Position{
		{X: initialX, Y: initialY}, // head
		{X: initialX - 1, Y: initialY},
		{X: initialX - 2, Y: initialY},
		{X: initialX - 3, Y: initialY}, // tail
	}

	m.tickCount = 0
	m.moveSpeed = INITIALSPEED
	m.snake = initialSnake
	m.direction = RIGHT
	m.dirChan = make(chan int, BUFFEREDDIRECTIONCHANGES)
	m.food = Position{X: initialX + 5, Y: initialY}
	m.score = 0
	m.gameOver = false
	m.pause = false
}

func (m *Model) updateSpeed() {
	newSpeed := INITIALSPEED - (m.score / 2)
	if newSpeed < 3 {
		newSpeed = 3
	}
	m.moveSpeed = newSpeed
}

func (m Model) tick() tea.Cmd {
	return tea.Every(TICKDURATION, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) newFoodPosition() Position {
	for {
		foodOnSnake := false
		food := Position{X: rand.Intn(m.boardWidth), Y: rand.Intn(m.boardHeight)}
		for _, pos := range m.snake {
			if pos.X == food.X && pos.Y == food.Y {
				foodOnSnake = true
				break
			}
		}
		if !foodOnSnake {
			return food
		}
	}
}

func (m Model) calcNewHead() Position {
	head := m.snake[0]

	switch m.direction {
	case UP:
		return Position{X: head.X, Y: head.Y - 1}
	case DOWN:
		return Position{X: head.X, Y: head.Y + 1}
	case LEFT:
		return Position{X: head.X - 1, Y: head.Y}
	case RIGHT:
		return Position{X: head.X + 1, Y: head.Y}
	default:
		return head
	}
}

func (m Model) checkCollision(pos Position) bool {
	if pos.X < 0 || pos.X >= m.boardWidth ||
		pos.Y < 0 || pos.Y >= m.boardHeight {
		return true
	}

	for _, bodyPos := range m.snake[1:] {
		if pos.X == bodyPos.X && pos.Y == bodyPos.Y {
			return true
		}
	}

	return false
}

func isOppositeDirection(a, b int) bool {
	return (a == UP && b == DOWN) ||
		(a == DOWN && b == UP) ||
		(a == LEFT && b == RIGHT) ||
		(a == RIGHT && b == LEFT)
}

func (m *Model) handleFood(newHead Position) {
	m.score++
	m.updateSpeed()
	m.food = m.newFoodPosition()
	m.snake = append([]Position{newHead}, m.snake...)
}

func (m *Model) handleTick() {
	m.tickCount++

	if m.tickCount >= m.moveSpeed {
		m.tickCount = 0
		lastValidDir := -1

	bufferLoop:
		for {
			select {
			case newDir := <-m.dirChan:
				if !isOppositeDirection(newDir, m.direction) && (newDir != m.direction) {
					log.Debug("New direction", "dir", newDir, "oldDir", m.direction)
					lastValidDir = newDir
				} else {
					log.Debug("Invalid direction", "dir", newDir, "oldDir", m.direction)
					continue bufferLoop
				}
			default:
			}

			if lastValidDir >= 0 {
				m.direction = lastValidDir
				m.lastDir = lastValidDir
				log.Debugf("New direction: %d", m.direction)
			}

			newHead := m.calcNewHead()

			if m.checkCollision(newHead) {
				m.gameOver = true
				return
			}

			if newHead.X == m.food.X && newHead.Y == m.food.Y {
				m.handleFood(newHead)
			} else {
				m.snake = append([]Position{newHead}, m.snake[:len(m.snake)-1]...)
			}

			break
		}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Height = msg.Height
		m.Width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "w", "k", "up":
			if m.lastDir == UP {
				break
			}
			select {
			case m.dirChan <- UP:
				m.lastDir = UP
				log.Debug("Direction UP")
			default:
				log.Warn("Buffer full, dropping up")
			}
		case "s", "j", "down":
			if m.lastDir == DOWN {
				break
			}
			select {
			case m.dirChan <- DOWN:
				m.lastDir = DOWN
				log.Debug("Direction DOWN")
			default:
				log.Warn("Buffer full, dropping down")
			}
		case "a", "h", "left":
			if m.lastDir == LEFT {
				break
			}
			select {
			case m.dirChan <- LEFT:
				m.lastDir = LEFT
				log.Debug("Direction LEFT")
			default:
				log.Warn("Buffer full, dropping left")
			}
		case "d", "l", "right":
			if m.lastDir == RIGHT {
				break
			}
			select {
			case m.dirChan <- RIGHT:
				m.lastDir = RIGHT
				log.Debug("Direction RIGHT")
			default:
				log.Warn("Buffer full, dropping right")
			}
		case " ":
			m.pause = !m.pause
		case "r":
			m.RestartGame()
		}
	case tickMsg:
		if m.pause {
			return m, m.tick()
		}

		if m.gameOver {
			return m, m.tick()
		}

		m.handleTick()
		return m, m.tick()
	}
	return m, nil
}

func (m Model) View() string {
	if m.gameOver {
		return m.QuitStyle.Render(fmt.Sprintf("Game Over! Score: %d\nPress 'r' to restart | Press 'q' to quit\n", m.score))
	}

	board := make([][]string, m.boardHeight)
	for i := range board {
		board[i] = make([]string, m.boardWidth)
		for j := range board[i] {
			board[i][j] = ""
		}
	}

	// Draw snake and food
	for _, pos := range m.snake[1:] {
		board[pos.Y][pos.X] = "S"
	}
	board[m.snake[0].Y][m.snake[0].X] = "H"
	board[m.food.Y][m.food.X] = "F"

	var s strings.Builder
	for y, row := range board {
		if y > 0 {
			s.WriteString("\n")
		}
		for _, cell := range row {
			var renderedCell string
			switch cell {
			case "H":
				renderedCell = "██"
				s.WriteString(m.SnakeStyle.Render(renderedCell))
			case "S":
				renderedCell = "▒▒"
				s.WriteString(m.SnakeStyle.Render(renderedCell))
			case "F":
				renderedCell = "🍎"
				s.WriteString(m.FoodStyle.Render(renderedCell))
			default:
				s.WriteString(m.GameBoardStyle.Render())
			}
		}
	}

	return lipgloss.Place(
		m.Width, m.Height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			m.TxtStyle.Render(s.String())+"\n",
			m.QuitStyle.Render("Press 'q' to quit"),
		),
	)
}
