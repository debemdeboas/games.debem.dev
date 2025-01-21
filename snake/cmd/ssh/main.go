package main

import (
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	snake "github.com/debemdeboas/games.debem.dev/snake/game"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"golang.org/x/net/context"
)

const (
	host = "0.0.0.0"
	port = "23232"
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

	m := snake.NewModel(
		pty.Term,
		renderer.ColorProfile().Name(),
		pty.Window.Width,
		pty.Window.Height,
		bg,
		txtStyle,
		quitStyle,
		foodStyle,
		snakeStyle,
		borderStyle,
	)

	return m, []tea.ProgramOption{tea.WithAltScreen()}
}
