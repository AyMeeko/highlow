package main

import (
  "html/template"
  "io"
  "net/http"

  Highlow "highlow/cmd/highlow"

  "github.com/labstack/echo/v4"
  "github.com/labstack/echo/v4/middleware"
  "github.com/google/uuid"
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

type GameSession struct {
  Games map[string]Game
}

type Game struct {
  DisplayName string
  ActiveCard string
  NextCard string
  UserChoiceLower string
  UserChoiceHigher string
  PlaceholderClass string
  GameClass string
  Verdict string
  State string
  Dirty bool
}

type Result struct {
  DisplayName string
  Text string
}

func initializeSingleGame() Game {
  DisplayName := uuid.New().String()
  return Game {
    DisplayName: DisplayName,
    PlaceholderClass: "",
    GameClass: "hide-game",
    State: "not_started",
    Dirty: false,
  }
}

func initializeGames(maxGames int) map[string]Game {
  games := map[string]Game{}
  for i := 0; i < maxGames; i++ {
    game := initializeSingleGame()
    games[game.DisplayName] = game
  }
  return games
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
  gameSession := GameSession{
    Games: initializeGames(4),
  }

  e := echo.New()
  e.Use(middleware.Logger())

  e.Renderer = newTemplate()

  // Browser routes
  e.Static("/css", "css")
  e.GET("/", func(c echo.Context) error {
    return c.Render(200, "index", gameSession)
  })

  e.GET("/game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    game, ok := gameSession.Games[displayName]
    if !ok {
      return c.Render(500, "game", game)
    }
    switch game.State {
    case "not_started":
      return c.Render(200, "game", game)
    case "in_progress":
      if !isUUID(game.DisplayName) && game.PlaceholderClass == "" {
        game.PlaceholderClass = "hide-placeholder"
        game.GameClass = ""
        delete(gameSession.Games, displayName)
      }
      if game.Dirty {
        // htmx wont actually re-render unless the gameSession object itself has changed
        // even if the underlying objects have changed.
        game.Dirty = false
        gameSession.Games[game.DisplayName] = game
      }
      return c.Render(200, "game", game)
    case "displaying_choice":
      game.State = "displaying_result"
      game.Dirty = false
      gameSession.Games[game.DisplayName] = game
      return c.Render(200, "game", game)
    case "displaying_result":
      game.UserChoiceLower = ""
      game.UserChoiceHigher = ""
      result := Result {
        DisplayName: displayName,
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
      game.Dirty = false
      gameSession.Games[game.DisplayName] = game
      return c.Render(200, "result", result)
    case "won":
      result := Result {
        DisplayName: displayName,
        Text: "You win!!",
      }
      return c.Render(200, "endGame", result)
    case "lost":
      result := Result {
        DisplayName: displayName,
        Text: "You lost!",
      }
      return c.Render(200, "endGame", result)
    default:
      return c.Render(500, "game", game)
    }
  })

  e.POST("/install-placeholder", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    game, ok := gameSession.Games[displayName]
    if !ok {
      return c.Render(500, "game", game)
    }
    if !isUUID(game.DisplayName) {
      game = initializeSingleGame()
      delete(gameSession.Games, displayName)
      game.Dirty = false
      gameSession.Games[game.DisplayName] = game
    }
    return c.Render(200, "game", game)
  })

  // HighLow game routes
  e.POST("/new-game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    _, ok := gameSession.Games[displayName]
    if ok {
      return c.Render(500, "gameSession", gameSession)
    }
    emptyGame := findEmptyGame(gameSession.Games)
    game := Game {
      DisplayName: displayName,
      ActiveCard: c.QueryParam("ActiveCard"),
      State: "in_progress",
      Dirty: true,
    }
    gameSession.Games[emptyGame.DisplayName] = game
    return c.String(http.StatusOK, "OK")
  })

  e.POST("/game", func(c echo.Context) error {
    displayName := c.QueryParam("DisplayName")
    game := gameSession.Games[displayName]
    game.DisplayName = displayName
    game.ActiveCard = c.QueryParam("ActiveCard")
    game.NextCard = c.QueryParam("NextCard")
    game.State = "displaying_choice"
    game.Verdict = c.QueryParam("Verdict")
    userChoice := c.QueryParam("UserChoice")
    if userChoice == "h" {
      game.UserChoiceHigher = "choice-higher"
      game.UserChoiceLower = ""
    } else if userChoice == "l" {
      game.UserChoiceLower = "choice-lower"
      game.UserChoiceHigher = ""
    }
    game.Dirty = true
    gameSession.Games[displayName] = game
    return c.String(http.StatusOK, "OK")
  })

  e.Logger.Fatal(e.Start(":42069"))
}
