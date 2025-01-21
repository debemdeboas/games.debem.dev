package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"golang.org/x/exp/rand"
	"golang.org/x/net/context"
)

const (
	host = "0.0.0.0"
	port = "23232"
)

const (
	tickDuration = 16 * time.Millisecond
	initialSpeed = 8

	up    = 0
	down  = 1
	left  = 2
	right = 3

	boardWidth  = 26
	boardHeight = 34

	bufferedDirectionChanges = 5
)

func main() {
	log.SetLevel(log.DebugLevel)

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath("host.key"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Server error", "error", err)
			done <- nil
		}
	}()
	<-done

	log.Info("Shutting down SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Server shutdown error", "error", err)
	}
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()

	renderer := bubbletea.MakeRenderer(s)
	txtStyle := renderer.NewStyle().Foreground(lipgloss.Color("10")).BorderStyle(lipgloss.RoundedBorder())
	quitStyle := renderer.NewStyle().Foreground(lipgloss.Color("8"))
	foodStyle := renderer.NewStyle().Foreground(lipgloss.Color("9"))
	snakeStyle := renderer.NewStyle().Foreground(lipgloss.Color("10"))
	borderStyle := renderer.NewStyle()

	bg := "light"
	if renderer.HasDarkBackground() {
		bg = "dark"
	}

	m := GameModel{
		term:        pty.Term,
		profile:     renderer.ColorProfile().Name(),
		width:       pty.Window.Width,
		height:      pty.Window.Height,
		bg:          bg,
		txtStyle:    txtStyle,
		quitStyle:   quitStyle,
		foodStyle:   foodStyle,
		snakeStyle:  snakeStyle,
		borderStyle: borderStyle,

		// Board
		boardWidth:  boardWidth,
		boardHeight: boardHeight,
		offsetX:     (boardWidth - pty.Window.Width) / 2,
		offsetY:     (boardHeight - pty.Window.Height) / 2,
	}
	m.restartGame()

	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

type Position struct {
	x, y int
}

type GameModel struct {
	term    string
	profile string
	width   int
	height  int
	bg      string

	// Styles
	txtStyle    lipgloss.Style
	quitStyle   lipgloss.Style
	foodStyle   lipgloss.Style
	snakeStyle  lipgloss.Style
	borderStyle lipgloss.Style

	// Game state
	tickCount int
	moveSpeed int
	snake     []Position
	direction int
	dirChan   chan int
	food      Position
	score     int
	gameOver  bool
	lastTick  time.Time
	pause     bool

	// Board
	boardWidth  int
	boardHeight int
	offsetX     int
	offsetY     int
}

type tickMsg time.Time

func (m GameModel) Init() tea.Cmd {
	return m.tick()
}

func (m *GameModel) restartGame() {
	initialX := boardWidth / 2
	initialY := boardHeight / 2

	initialSnake := []Position{
		{x: initialX, y: initialY}, // head
		{x: initialX - (1 * 1), y: initialY},
		{x: initialX - (2 * 1), y: initialY},
		{x: initialX - (3 * 1), y: initialY}, // tail
	}

	m.tickCount = 0
	m.moveSpeed = initialSpeed
	m.snake = initialSnake
	m.direction = right
	m.dirChan = make(chan int, bufferedDirectionChanges)
	m.food = Position{x: initialX + 5, y: initialY}
	m.score = 0
	m.gameOver = false
	m.lastTick = time.Now()
	m.pause = false
}

func (m *GameModel) updateSpeed() {
	newSpeed := initialSpeed - (m.score / 5)
	if newSpeed < 4 {
		newSpeed = 4
	}
	m.moveSpeed = newSpeed
}

func (m GameModel) tick() tea.Cmd {
	return tea.Every(tickDuration, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m GameModel) newFoodPosition() Position {
	// Generate new food position randomly within the board
	// Ensure food doesn't spawn on the snake
	for {
		foodOnSnake := false
		food := Position{x: rand.Intn(m.boardWidth), y: rand.Intn(m.boardHeight)}
		for _, pos := range m.snake {
			if pos.x == food.x && pos.y == food.y {
				log.Info("Food spawned on snake", "food", food, "snake", m.snake)
				foodOnSnake = true
				break
			}
		}
		if !foodOnSnake {
			return food
		}
	}
}

func (m *GameModel) handleFood(newHead Position) {
	m.score++
	m.updateSpeed()

	m.food = m.newFoodPosition()
	log.Debug("New food position", "food", m.food)

	// Grow snake
	m.snake = append([]Position{newHead}, m.snake...)
}

func (m *GameModel) handleTick() {
	m.tickCount = 0

	// Move snake
	head := m.snake[0]
	newHead := Position{x: head.x, y: head.y}

	select {
	case newDir := <-m.dirChan:
		m.direction = newDir
	default:
	}

	switch m.direction {
	case up:
		newHead.y -= 1
	case down:
		newHead.y += 1
	case left:
		newHead.x -= 1
	case right:
		newHead.x += 1
	}

	// Check for collision with borders
	if newHead.x < 0 || newHead.x >= m.boardWidth ||
		newHead.y < 0 || newHead.y >= m.boardHeight {
		log.Debug("Snake collided with border", "head", newHead)
		m.gameOver = true
		return
	}

	// Check for collision with self
	for _, pos := range m.snake[1:] {
		if newHead.x == pos.x && newHead.y == pos.y {
			log.Debug("Snake collided with itself", "head", newHead, "pos", pos)
			m.gameOver = true
			return
		}
	}

	// Check for food
	if newHead.x == m.food.x && newHead.y == m.food.y {
		log.Info("Snake ate food", "head", newHead, "food", m.food)
		m.handleFood(newHead)
	} else {
		m.snake = append([]Position{newHead}, m.snake[:len(m.snake)-1]...)
	}
}

func (m GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "w", "k", "up":
			if m.direction != down && m.direction != up {
				log.Debug("Changing direction to up")
				select {
				case m.dirChan <- up:
				default:
				}
			}
		case "s", "j", "down":
			if m.direction != up && m.direction != down {
				log.Debug("Changing direction to down")
				select {
				case m.dirChan <- down:
				default:
				}
			}
		case "a", "h", "left":
			if m.direction != right && m.direction != left {
				log.Debug("Changing direction to left")
				select {
				case m.dirChan <- left:
				default:
				}
			}
		case "d", "l", "right":
			if m.direction != left && m.direction != right {
				log.Debug("Changing direction to right")
				select {
				case m.dirChan <- right:
				default:
				}
			}
		case " ":
			m.pause = !m.pause
		case "r":
			m.restartGame()
		}
	case tickMsg:
		if m.pause {
			return m, m.tick()
		}

		if m.gameOver {
			return m, m.tick()
		}

		m.tickCount++
		if m.tickCount >= m.moveSpeed {
			m.handleTick()
		}
		return m, m.tick()
	}
	return m, nil
}

func (m GameModel) View() string {
	if m.gameOver {
		return m.quitStyle.Render(fmt.Sprintf("Game Over! Score: %d\nPress 'r' to restart | Press 'q' to quit\n", m.score))
	}

	board := make([][]string, m.boardHeight)
	for i := range board {
		board[i] = make([]string, m.boardWidth)
		for j := range board[i] {
			board[i][j] = ""
		}
	}

	// Draw snake and food on logical board
	for _, pos := range m.snake[1:] {
		board[pos.y][pos.x] = "S"
	}
	board[m.snake[0].y][m.snake[0].x] = "H"

	board[m.food.y][m.food.x] = "F"

	// Build the view with doubled horizontal width
	var s strings.Builder
	for y, row := range board {
		if y > 0 {
			s.WriteString("\n")
		}
		for _, cell := range row {
			var renderedCell string
			switch cell {
			case "H": // Head
				renderedCell = "██"
				s.WriteString(m.snakeStyle.Render(renderedCell))
			case "S": // Double snake
				renderedCell = "▒▒"
				s.WriteString(m.snakeStyle.Render(renderedCell))
			case "F": // Double food
				renderedCell = "🍎"
				s.WriteString(m.foodStyle.Render(renderedCell))
			default:
				renderedCell = "  "
				s.WriteString(m.borderStyle.Render(renderedCell))
			}
		}
	}

	return m.txtStyle.Render(s.String()) + "\n" +
		m.quitStyle.Render(fmt.Sprintf("Score: %d | Press 'r' to restart | Press 'q' to quit\n", m.score))
}
