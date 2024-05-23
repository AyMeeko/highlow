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

type Player struct {
  displayName string
}

type Game struct {
  player *Player
  deck *Deck
}

type Deck struct {
  cards []int
  pointer int
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
  fmt.Printf("Shuffled deck: %d\n", deckCards)
  return deckCards
}

func createGame(displayName string) *Game {
  player := Player{
    displayName: displayName,
  }

  deck := Deck{
    cards: createAndShuffleDeck(),
    pointer: 0,
  }

  return &Game{
    player: &player,
    deck: &deck,
  }
}

func handleMessage(restClient *resty.Client, session *map[string]*Game, displayName, message string) {
  if message == "!shutdown" && displayName == "AyMeeko" {
    fmt.Println("Shutting down server...")
    os.Exit(0)
  }

  if message == "!j" {
    if len(*session) == 4 {
      fmt.Println("Sorry, reached max number of players.")
    }
    game, ok := (*session)[displayName]
    if !ok {
      game = createGame(displayName)
      (*session)[displayName] = game

      fmt.Printf("Game started for %s. Active card: %d\n", displayName, game.deck.cards[game.deck.pointer])
      triggerNewGame(restClient, displayName, game.deck.cards[game.deck.pointer])
    }
  } else if message == "h" || message == "l" {
    game, ok := (*session)[displayName]
    if ok {
      activeCard := game.deck.cards[game.deck.pointer]
      nextCard := game.deck.cards[game.deck.pointer + 1]
      result := (message == "h" && nextCard > activeCard) || (message == "l" && nextCard < activeCard)
      if result {
        game.deck.pointer += 1
        if game.deck.pointer == len(game.deck.cards)-1 {
          fmt.Println("Correct! You win!!")
          delete((*session), displayName)
          triggerGameUpdate(restClient, displayName, activeCard, message, "won", nextCard)
        } else {
          fmt.Printf("Correct! Active card: %d\n", nextCard)
          triggerGameUpdate(restClient, displayName, activeCard, message, "correct", nextCard)
        }
      } else {
        fmt.Printf("Incorrect! The next card was %d. Better luck next time!\n", nextCard)
        delete((*session), displayName)
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
  session := make(map[string]*Game)
  channel := make(chan ChannelMessage)
  rateLimit := make(map[string]bool)

  client := twitch.NewAnonymousClient()

  client.OnPrivateMessage(func(rawMessage twitch.PrivateMessage) {
    displayName := rawMessage.User.DisplayName
    message := strings.ToLower(rawMessage.Message)

    go func() {
      if rateLimit[displayName] {
        triggerNotificationUpdate(
          restClient,
          displayName,
          "Fastest fingers in the west! Slow down with your messages.",
        )
        session[displayName].player.rateLimited = true
        fmt.Printf("Rate limited %s\n", displayName)
      } else {
        channel <- ChannelMessage {
          DisplayName: displayName,
          RateLimited: true,
        }
        handleMessage(restClient, &session, displayName, message)
        time.Sleep(2*time.Second)
        if session[displayName].player.rateLimited {
          session[displayName].player.rateLimited = false
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
