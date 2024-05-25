/*
$ go run ./cmd/highlow/game.go
*/
package main

import (
  "fmt"
  "math/rand"
  "net/url"
  "os"
  "strings"
  "sync"
  "time"

  "github.com/gempir/go-twitch-irc/v4"
  "github.com/go-resty/resty/v2"
)

const baseUrl = "http://localhost:42069"
const enableLogging = false
const gameExpirationTime = 5 * time.Minute
const rateLimitDuration = 3 * time.Second
var restClient = resty.New()


type PlayerSession struct {
  ActiveGame *Game
  HiScore int
  NumGames int
  NumWins int
  RateLimited bool
  LimiterCountdown *time.Timer
  Lock sync.RWMutex
}

type Game struct {
  Deck *Deck
  DisplayName string
  Score int
  Timeout *time.Timer
}

type Deck struct {
  Cards []int
  Pointer int
}

func createLimiterCountdown(displayName string) *time.Timer {
  duration := 5 * time.Second
  return time.AfterFunc(duration, func() {
    triggerNotificationUpdate(displayName, "")
  })
}

func createGameTimeoutCountdown(playerSession *PlayerSession) *time.Timer {
  return time.AfterFunc(gameExpirationTime, func() {
    displayName := playerSession.ActiveGame.DisplayName
    playerSession.Lock.Lock()
    playerSession.ActiveGame = nil
    playerSession.Lock.Unlock()
    triggerGameExpiration(displayName)
  })
}

func createAndShuffleDeck() []int {
  deckCards := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
  rand.Shuffle(len(deckCards), func(i, j int) {
    deckCards[i], deckCards[j] = deckCards[j], deckCards[i]
  })
  return deckCards
}

func createPlayerSession() *PlayerSession {
  return &PlayerSession {
    HiScore: 0,
    NumGames: 0,
    NumWins: 0,
    RateLimited: false,
  }
}

func createGame(displayName string) *Game {
  deck := Deck{
    Cards: createAndShuffleDeck(),
    Pointer: 0,
  }
  fmt.Printf("Shuffled deck for %s: %d\n", displayName, deck.Cards)

  return &Game{
    DisplayName: displayName,
    Deck: &deck,
    Score: 0,
  }
}

func handleMessage(session *map[string]*PlayerSession, displayName, message string) {
  if message == "!shutdown" && displayName == "AyMeeko" {
    fmt.Println("Shutting down server...")
    targetUrl := fmt.Sprintf("http://localhost:42069/shut-down")
    restClient.R().Post(targetUrl)
    os.Exit(0)
  }

  playerSession := (*session)[displayName]
  if message == "!j" {
    if len(*session) == 4 {
      fmt.Println("Sorry, reached max number of players.")
    }
    if playerSession.ActiveGame == nil {
      game := createGame(displayName)
      playerSession.ActiveGame = game
      game.Timeout = createGameTimeoutCountdown(playerSession)
      (*session)[displayName] = playerSession
      fmt.Printf("Game started for %s. Active card: %d\n", displayName, game.Deck.Cards[game.Deck.Pointer])
      triggerNewGame(playerSession, displayName, game.Deck.Cards[game.Deck.Pointer])
    }
  } else if message == "h" || message == "l" {
    game := playerSession.ActiveGame
    if game != nil {
      game.Timeout.Stop()
      activeCard := game.Deck.Cards[game.Deck.Pointer]
      nextCard := game.Deck.Cards[game.Deck.Pointer + 1]
      result := (message == "h" && nextCard > activeCard) || (message == "l" && nextCard < activeCard)
      if result {
        game.Deck.Pointer += 1
        game.Score += 1
        if game.Deck.Pointer == len(game.Deck.Cards)-1 {
          fmt.Println("Correct! You win!!")
          playerSession.ActiveGame = nil
          playerSession.NumGames += 1
          playerSession.NumWins += 1
          if playerSession.HiScore < game.Score {
            playerSession.HiScore = game.Score
          }
          triggerGameUpdate(playerSession, displayName, activeCard, message, "won", nextCard, game.Score)
        } else {
          fmt.Printf("Correct! Active card: %d\n", nextCard)
          triggerGameUpdate(playerSession, displayName, activeCard, message, "correct", nextCard, game.Score)
          game.Timeout = createGameTimeoutCountdown(playerSession)
        }
      } else {
        fmt.Printf("Incorrect! The next card was %d. Better luck next time!\n", nextCard)
        playerSession.ActiveGame = nil
        playerSession.NumGames += 1
        if playerSession.HiScore < game.Score {
          playerSession.HiScore = game.Score
        }
        triggerGameUpdate(playerSession, displayName, activeCard, message, "lost", nextCard, game.Score)
      }
    }
  }
}

func sendPost(body, targetUrl string) {
  restClient.R().SetHeader("Content-Type", "application/x-www-form-urlencoded").SetBody(body).Post(targetUrl)
}

func triggerNewGame(playerSession *PlayerSession, displayName string, activeCard int) {
  targetUrl := fmt.Sprintf("%s/new-game", baseUrl)
  body := fmt.Sprintf(
    "DisplayName=%s&ActiveCard=%d&HiScore=%d&NumGames=%d&NumWins=%d",
    displayName,
    activeCard,
    playerSession.HiScore,
    playerSession.NumGames,
    playerSession.NumWins,
  )
  sendPost(body, targetUrl)
}

func triggerGameUpdate(playerSession *PlayerSession, displayName string, activeCard int, userChoice string, verdict string, nextCard int, score int) {
  targetUrl := fmt.Sprintf("%s/game", baseUrl)
  body := fmt.Sprintf(
    "DisplayName=%s&ActiveCard=%d&UserChoice=%s&Verdict=%s&NextCard=%d&HiScore=%d&NumGames=%d&NumWins=%d&Score=%d",
    displayName,
    activeCard,
    userChoice,
    verdict,
    nextCard,
    playerSession.HiScore,
    playerSession.NumGames,
    playerSession.NumWins,
    score,
  )
  sendPost(body, targetUrl)
}

func triggerNotificationUpdate(displayName string, text string) {
  targetUrl := fmt.Sprintf("%s/update-notification", baseUrl)
  body := fmt.Sprintf(
    "DisplayName=%s&NotificationText=%s",
    displayName,
    url.QueryEscape(text),
  )
  sendPost(body, targetUrl)
}

func triggerGameExpiration(displayName string) {
  targetUrl := fmt.Sprintf("%s/expire-game", baseUrl)
  body := fmt.Sprintf("DisplayName=%s", displayName)
  sendPost(body, targetUrl)
}

func log(text string) {
  if enableLogging {
    fmt.Printf("[DEBUG] %s %s\n", time.Now().Format("2006-01-02T15:04:05"), text)
  }
}

func main() {
  session := make(map[string]*PlayerSession)

  client := twitch.NewAnonymousClient()

  client.OnPrivateMessage(func(rawMessage twitch.PrivateMessage) {
    displayName := rawMessage.User.DisplayName
    message := strings.ToLower(rawMessage.Message)

    playerSession, ok := session[displayName]
    if !ok {
      playerSession = createPlayerSession()
      session[displayName] = playerSession
    }

    go func() {
      log(fmt.Sprintf("Before, RateLimited: %t", playerSession.RateLimited))
      if playerSession.RateLimited {
        triggerNotificationUpdate(
          displayName,
          "Fastest fingers in the west! Slow down with your messages.",
        )
        playerSession.LimiterCountdown = createLimiterCountdown(displayName)
        log(fmt.Sprintf("RateLimited: %s", displayName))
      } else {
        playerSession.Lock.Lock()
        log("Locking")
        playerSession.RateLimited = true
        handleMessage(&session, displayName, message)
        time.Sleep(rateLimitDuration)
        playerSession.RateLimited = false
        log(fmt.Sprintf("Inside, RateLimited: %t", playerSession.RateLimited))
        log("Unlocking")
        playerSession.Lock.Unlock()
      }
    }()
  })

  client.Join("AyMeeko")

  err := client.Connect()
  if err != nil {
    panic(err)
  }
}
