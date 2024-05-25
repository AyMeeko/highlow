package main

import (
  "context"
  "html/template"
  "io"
  "net/http"
  "os"
  "os/signal"
  "sync"
  "syscall"
  "time"

  "github.com/labstack/echo/v4"
  "github.com/labstack/echo/v4/middleware"
)

type Templates struct {
  templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
  return t.templates.ExecuteTemplate(w, name, data)
}

func newTemplate() *Templates {
  return &Templates{
    templates: template.Must(template.ParseGlob("views/*.html")),
  }
}

type Session struct {
  Mapping map[string]*PlayerSession
  lock sync.RWMutex
}

func (s *Session) Read(displayName string) (*PlayerSession, bool) {
  s.lock.RLock()
  found, ok := s.Mapping[displayName]
  s.lock.RUnlock()
  return found, ok
}

func (s *Session) Write(displayName string, playerSession *PlayerSession) {
  s.lock.Lock()
  s.Mapping[displayName] = playerSession
  s.lock.Unlock()
}

func (s *Session) FindUnrendered() *Game {
  s.lock.Lock()
  defer s.lock.Unlock()
  for _, playerSession := range s.Mapping {
    game := playerSession.ActiveGame
    if game != nil && !game.Rendered {
      game.Rendered = true
      return game
    }
  }
  return nil
}

type PlayerSession struct {
  DisplayName string
  ActiveGame *Game
  HiScore int
  NumGames int
  NumWins int
}

type Game struct {
  DisplayName string
  ActiveCard string
  NextCard string
  Verdict string
  State string
  UserChoiceLowerClass string
  UserChoiceHigherClass string
  HideNotificationTextClass string
  NotificationText string
  Rendered bool
}

type Placeholder struct {
  visible bool
}

type Result struct {
  DisplayName string
  Text string
  HideNotificationTextClass string
  NotificationText string
}

func initializePlayerSession(displayName string) *PlayerSession {
  return &PlayerSession {
    DisplayName: displayName,
    HiScore: 0,
    NumGames: 0,
    NumWins: 0,
  }
}

func initializeGame(displayName string, activeCard string) *Game {
  return &Game {
    DisplayName: displayName,
    ActiveCard: activeCard,
    State: "in_progress",
    Rendered: false,
    HideNotificationTextClass: "hide-notification",
  }
}

func main() {
  placeholder := Placeholder {
    visible: true,
  }
  session := Session {
    Mapping: make(map[string]*PlayerSession),
  }

  e := echo.New()
  e.Use(middleware.Logger())

  e.Renderer = newTemplate()

  // Browser routes
  e.Static("/css", "css")
  e.GET("/", func(c echo.Context) error {
    return c.Render(200, "index", session.Mapping)
  })

  e.GET("/game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok || playerSession.ActiveGame == nil {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }

    game := playerSession.ActiveGame
    switch game.State {
    case "not_started":
      return c.Render(200, "game", game)
    case "in_progress":
      return c.Render(200, "game", game)
    case "displaying_choice":
      game.State = "displaying_result"
      return c.Render(200, "game", game)
    case "displaying_result":
      game.UserChoiceLowerClass = ""
      game.UserChoiceHigherClass = ""
      result := Result {
        DisplayName: displayName,
        NotificationText: game.NotificationText,
        HideNotificationTextClass: game.HideNotificationTextClass,
      }
      switch game.Verdict {
      case "correct":
        game.State = "in_progress"
        result.Text = "Correct!"
        game.ActiveCard = game.NextCard
      case "won":
        game.State = "won"
        result.Text = "Correct!"
      default:
        game.State = "lost"
        result.Text = "Incorrect!"
      }
      return c.Render(200, "result", result)
    case "won":
      result := Result {
        DisplayName: displayName,
        Text: "You win!!",
        NotificationText: game.NotificationText,
        HideNotificationTextClass: game.HideNotificationTextClass,
      }
      return c.Render(200, "endGame", result)
    case "lost":
      result := Result {
        DisplayName: displayName,
        Text: "You lost!",
        NotificationText: game.NotificationText,
        HideNotificationTextClass: game.HideNotificationTextClass,
      }
      return c.Render(200, "endGame", result)
    case "expired":
      result := Result {
        DisplayName: displayName,
        Text: "Game expired.",
      }
      return c.Render(200, "endGame", result)
    default:
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
  })

  e.POST("/install-placeholder", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    playerSession.ActiveGame = nil
    return c.Render(200, "placeholder", placeholder)
  })

  e.GET("/check-for-new-game", func(c echo.Context) error {
    game := session.FindUnrendered()
    if game != nil {
      return c.Render(200, "game", game)
    }
    return c.Render(200, "placeholder", placeholder)
  })

  // HighLow game routes
  e.POST("/new-game", func(c echo.Context) error {
    displayName := c.FormValue("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok {
      playerSession = initializePlayerSession(displayName)
      session.Write(displayName, playerSession)
    }
    if playerSession.ActiveGame != nil {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    playerSession.ActiveGame = initializeGame(displayName, c.FormValue("ActiveCard"))

    return c.String(http.StatusOK, "OK")
  })

  e.POST("/game", func(c echo.Context) error {
    displayName := c.FormValue("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok || (ok && playerSession.ActiveGame == nil) {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    game := playerSession.ActiveGame
    game.ActiveCard = c.FormValue("ActiveCard")
    game.NextCard = c.FormValue("NextCard")
    game.State = "displaying_choice"
    game.Verdict = c.FormValue("Verdict")
    userChoice := c.FormValue("UserChoice")
    if userChoice == "h" {
      game.UserChoiceHigherClass = "choice-higher"
      game.UserChoiceLowerClass = ""
    } else if userChoice == "l" {
      game.UserChoiceLowerClass = "choice-lower"
      game.UserChoiceHigherClass = ""
    }
    return c.String(http.StatusOK, "OK")
  })

  e.POST("/update-notification", func(c echo.Context) error {
    displayName := c.FormValue("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok || (ok && playerSession.ActiveGame == nil) {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    game := playerSession.ActiveGame
    game.NotificationText = c.FormValue("NotificationText")
    if game.NotificationText == "" {
      game.HideNotificationTextClass = "hide-notification"
    } else {
      game.HideNotificationTextClass = ""
    }
    return c.String(http.StatusOK, "OK")
  })

  e.POST("/expire-game", func(c echo.Context) error {
    displayName := c.FormValue("DisplayName")
    playerSession, ok := session.Read(displayName)

    if !ok || (ok && playerSession.ActiveGame == nil) {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    game := playerSession.ActiveGame
    if game != nil {
      game.State = "expired"
    }
    return c.String(http.StatusOK, "OK")
  })

  e.POST("/shut-down", func(c echo.Context) error {
    syscall.Kill(syscall.Getpid(), syscall.SIGINT)
    return c.String(http.StatusOK, "OK")
  })

  e.Logger.Fatal(e.Start(":42069"))

  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
  defer stop()
  // Start server
  go func() {
    if err := e.Start(":42069"); err != nil && err != http.ErrServerClosed {
      e.Logger.Fatal("shutting down the server")
    }
  }()

  // Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
  <-ctx.Done()
  ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
  defer cancel()
  if err := e.Shutdown(ctx); err != nil {
    e.Logger.Fatal(err)
  }
}
