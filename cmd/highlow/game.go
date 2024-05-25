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
  Streak int
  DisplayName string
  Score int
  Timeout *time.Timer
}

type Deck struct {
  Cards []int
  Pointer int
}

func createLimiterCountdown(restClient *resty.Client, displayName string) *time.Timer {
  duration := 5 * time.Second
  return time.AfterFunc(duration, func() {
    triggerNotificationUpdate(restClient, displayName, "")
  })
}

func createGameTimeoutCountdown(restClient *resty.Client, displayName string) *time.Timer {
  return time.AfterFunc(gameExpirationTime, func() {
    triggerGameExpiration(restClient, displayName)
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

func handleMessage(restClient *resty.Client, session *map[string]*PlayerSession, displayName, message string) {
  if message == "!shutdown" && displayName == "AyMeeko" {
    fmt.Println("Shutting down server...")
    targetUrl := fmt.Sprintf("http://localhost:42069/shut-down")
    restClient.R().EnableTrace().Post(targetUrl)
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
      game.Timeout = createGameTimeoutCountdown(restClient, displayName)
      (*session)[displayName] = playerSession
      fmt.Printf("Game started for %s. Active card: %d\n", displayName, game.Deck.Cards[game.Deck.Pointer])
      triggerNewGame(restClient, displayName, game.Deck.Cards[game.Deck.Pointer])
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
          triggerGameUpdate(restClient, displayName, activeCard, message, "won", nextCard)
        } else {
          fmt.Printf("Correct! Active card: %d\n", nextCard)
          triggerGameUpdate(restClient, displayName, activeCard, message, "correct", nextCard)
          game.Timeout = createGameTimeoutCountdown(restClient, displayName)
        }
      } else {
        fmt.Printf("Incorrect! The next card was %d. Better luck next time!\n", nextCard)
        playerSession.ActiveGame = nil
        playerSession.NumGames += 1
        if playerSession.HiScore < game.Score {
          playerSession.HiScore = game.Score
        }
        triggerGameUpdate(restClient, displayName, activeCard, message, "lost", nextCard)
      }
    }
  }
}

func triggerNewGame(restClient *resty.Client, displayName string, activeCard int) {
  targetUrl := fmt.Sprintf(
    "%s/new-game?DisplayName=%s&ActiveCard=%d",
    baseUrl,
    displayName,
    activeCard,
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func triggerGameUpdate(restClient *resty.Client, displayName string, activeCard int, userChoice string, verdict string, nextCard int) {
  targetUrl := fmt.Sprintf(
    "%s/game?DisplayName=%s&ActiveCard=%d&UserChoice=%s&Verdict=%s&NextCard=%d",
    baseUrl,
    displayName,
    activeCard,
    userChoice,
    verdict,
    nextCard,
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func triggerNotificationUpdate(restClient *resty.Client, displayName string, text string) {
  targetUrl := fmt.Sprintf(
    "%s/update-notification?DisplayName=%s&NotificationText=%s",
    baseUrl,
    displayName,
    url.QueryEscape(text),
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func triggerGameExpiration(restClient *resty.Client, displayName string) {
  targetUrl := fmt.Sprintf(
    "%s/expire-game?DisplayName=%s",
    baseUrl,
    displayName,
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func log(text string) {
  if enableLogging {
    fmt.Printf("[DEBUG] %s %s\n", time.Now().Format("2006-01-02T15:04:05"), text)
  }
}

func main() {
  restClient := resty.New()
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
          restClient,
          displayName,
          "Fastest fingers in the west! Slow down with your messages.",
        )
        playerSession.LimiterCountdown = createLimiterCountdown(restClient, displayName)
        log(fmt.Sprintf("RateLimited: %s", displayName))
      } else {
        playerSession.Lock.Lock()
        log("Locking")
        playerSession.RateLimited = true
        handleMessage(restClient, &session, displayName, message)
        time.Sleep(3 * time.Second)
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
