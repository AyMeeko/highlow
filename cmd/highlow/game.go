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
  "time"

  "github.com/gempir/go-twitch-irc/v4"
  "github.com/go-resty/resty/v2"
)

type PlayerSession struct {
  ActiveGame *Game
  HiScore int
  NumGames int
  NumWins int
  RateLimited bool
}

type Game struct {
  Deck *Deck
  Streak int
  DisplayName string
  Score int
}

type Deck struct {
  Cards []int
  Pointer int
}

type ChannelMessage struct {
  DisplayName string
  RateLimited bool
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
    os.Exit(0)
  }

  if message == "!j" {
    if len(*session) == 4 {
      fmt.Println("Sorry, reached max number of players.")
    }
    playerSession := (*session)[displayName]
    if playerSession.ActiveGame == nil {
      game := createGame(displayName)
      playerSession.ActiveGame = game
      (*session)[displayName] = playerSession
      fmt.Printf("Game started for %s. Active card: %d\n", displayName, game.Deck.Cards[game.Deck.Pointer])
      triggerNewGame(restClient, displayName, game.Deck.Cards[game.Deck.Pointer])
    }
  } else if message == "h" || message == "l" {
    playerSession, ok := (*session)[displayName]
    game := playerSession.ActiveGame
    if ok && game != nil {
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
    "http://localhost:42069/new-game?DisplayName=%s&ActiveCard=%d",
    displayName,
    activeCard,
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func triggerGameUpdate(restClient *resty.Client, displayName string, activeCard int, userChoice string, verdict string, nextCard int) {
  targetUrl := fmt.Sprintf(
    "http://localhost:42069/game?DisplayName=%s&ActiveCard=%d&UserChoice=%s&Verdict=%s&NextCard=%d",
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
    "http://localhost:42069/update-notification?DisplayName=%s&NotificationText=%s",
    displayName,
    url.QueryEscape(text),
  )
  restClient.R().EnableTrace().Post(targetUrl)
}

func main() {
  restClient := resty.New()
  session := make(map[string]*PlayerSession)
  channel := make(chan ChannelMessage)
  rateLimit := make(map[string]bool)

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
      if rateLimit[displayName] {
        triggerNotificationUpdate(
          restClient,
          displayName,
          "Fastest fingers in the west! Slow down with your messages.",
        )
        playerSession.RateLimited = true
        fmt.Printf("Rate limited %s\n", displayName)
      } else {
        channel <- ChannelMessage {
          DisplayName: displayName,
          RateLimited: true,
        }
        handleMessage(restClient, &session, displayName, message)
        time.Sleep(2*time.Second)
        if playerSession.RateLimited {
          //time.Sleep(2*time.Second)
          playerSession.RateLimited = false
          triggerNotificationUpdate(restClient, displayName, "")
        }
        channel <- ChannelMessage {
          DisplayName: displayName,
          RateLimited: false,
        }
      }
    }()
  })

  go func() {
    for {
      msg := <-channel
      rateLimit[msg.DisplayName] = msg.RateLimited
    }
  }()

  client.Join("AyMeeko")

  err := client.Connect()
  if err != nil {
    panic(err)
  }
}
