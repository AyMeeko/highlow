package main

import (
  "fmt"
  "html/template"
  "io"
  "net/http"

  "github.com/google/uuid"
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
  Dirty bool
  UserChoiceLowerClass string
  UserChoiceHigherClass string
  PlaceholderClass string
  GameClass string
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

func findEmptyGame(games map[string]Game) Game {
  var foundGame string
  for key, val := range games {
    if val.PlaceholderClass == "" {
      foundGame = key
      break
    }
  }
  return games[foundGame]
}

func isUUID(u string) bool {
  _, err := uuid.Parse(u)
  return err == nil
}

func main() {
  placeholder := Placeholder {
    visible: true,
  }
  session := make(map[string]*PlayerSession)

  e := echo.New()
  e.Use(middleware.Logger())

  e.Renderer = newTemplate()

  // Browser routes
  e.Static("/css", "css")
  e.GET("/", func(c echo.Context) error {
    return c.Render(200, "index", session)
  })

  e.GET("/game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session[displayName]
    game := playerSession.ActiveGame

    if !ok || game == nil {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
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
    default:
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
  })

  e.POST("/install-placeholder", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session[displayName]

    if !ok {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    playerSession.ActiveGame = nil
    return c.Render(200, "placeholder", placeholder)
  })

  e.GET("/check-for-new-game", func(c echo.Context) error {
    for _, playerSession := range session {
      game := playerSession.ActiveGame
      if game != nil {
        game.Rendered = true
        return c.Render(200, "game", game)
      }
    }
    return c.Render(200, "placeholder", placeholder)
  })

  // HighLow game routes
  e.POST("/new-game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session[displayName]

    if !ok {
      playerSession = initializePlayerSession(displayName)
      session[displayName] = playerSession
    }
    if playerSession.ActiveGame != nil {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    playerSession.ActiveGame = initializeGame(displayName, c.QueryParam("ActiveCard"))

    return c.String(http.StatusOK, "OK")
  })

  e.POST("/game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session[displayName]

    if !ok || (ok && playerSession.ActiveGame == nil) {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    game := playerSession.ActiveGame
    game.ActiveCard = c.QueryParam("ActiveCard")
    game.NextCard = c.QueryParam("NextCard")
    game.State = "displaying_choice"
    game.Verdict = c.QueryParam("Verdict")
    userChoice := c.QueryParam("UserChoice")
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
    displayName := c.QueryParam("DisplayName")
    playerSession, ok := session[displayName]

    if !ok || (ok && playerSession.ActiveGame == nil) {
      return c.String(http.StatusInternalServerError, "InternalServerError")
    }
    game := playerSession.ActiveGame
    game.NotificationText = c.QueryParam("NotificationText")
    if game.NotificationText == "" {
      game.HideNotificationTextClass = "hide-notification"
    } else {
      game.HideNotificationTextClass = ""
    }
    return c.String(http.StatusOK, "OK")
  })

  e.Logger.Fatal(e.Start(":42069"))
}
