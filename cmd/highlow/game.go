/*
$ go run ./cmd/highlow/game.go
*/
package main

import (
  "fmt"
  "math/rand"
  "os"

  "github.com/gempir/go-twitch-irc/v4"
  "github.com/go-resty/resty/v2"
)

type Player struct {
  id string
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

func createAndShuffleDeck() []int {
  deckCards := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
  rand.Shuffle(len(deckCards), func(i, j int) {
    deckCards[i], deckCards[j] = deckCards[j], deckCards[i]
  })
  fmt.Printf("Shuffled deck: %d\n", deckCards)
  return deckCards
}

func newGame(userId, displayName string) Game {
  player := Player{
    id: userId,
    displayName: displayName,
  }

  deck := Deck{
    cards: createAndShuffleDeck(),
    pointer: 0,
  }

  return Game{
    player: &player,
    deck: &deck,
  }
}

func HandleMessage(restClient *resty.Client, session map[string]Game, displayName, message, userId string) map[string]Game {
  if message == "!shutdown" && displayName == "AyMeeko" {
    fmt.Println("Shutting down server...")
    os.Exit(0)
  }
  if message == "!j" {
    if len(session) == 4 {
      fmt.Println("Sorry, reached max number of players.")
      return nil
    }
    game, ok := session[displayName]
    if !ok {
      game = newGame(userId, displayName)
      session[displayName] = game
    }
    fmt.Printf("Game started for %s. Active card: %d\n", displayName, game.deck.cards[game.deck.pointer])
    triggerNewGame(restClient, displayName, game.deck.cards[game.deck.pointer])
  } else if message == "h" || message == "l" {
    game, ok := session[displayName]
    if ok {
      activeCard := game.deck.cards[game.deck.pointer]
      nextCard := game.deck.cards[game.deck.pointer + 1]
      result := (message == "h" && nextCard > activeCard) || (message == "l" && nextCard < activeCard)
      if result == true {
        game.deck.pointer += 1
        if game.deck.pointer == len(game.deck.cards)-1 {
          fmt.Println("Correct! You win!!")
          delete(session, displayName)
          updateGame(restClient, displayName, activeCard, message, "won", nextCard)
        } else {
          fmt.Printf("Correct! Active card: %d\n", nextCard)
          updateGame(restClient, displayName, activeCard, message, "correct", nextCard)
        }
      } else {
        fmt.Printf("Incorrect! The next card was %d. Better luck next time!\n", nextCard)
        delete(session, displayName)
        updateGame(restClient, displayName, activeCard, message, "lost", nextCard)
      }
    }
  }
  return session
}

func triggerNewGame(restClient *resty.Client, displayName string, activeCard int) {
  url := fmt.Sprintf(
    "http://localhost:42069/new-game?DisplayName=%s&ActiveCard=%d",
    displayName,
    activeCard,
  )
  restClient.R().EnableTrace().Post(url)
}

func updateGame(restClient *resty.Client, displayName string, activeCard int, userChoice string, verdict string, nextCard int) {
  url := fmt.Sprintf(
    "http://localhost:42069/game?DisplayName=%s&ActiveCard=%d&UserChoice=%s&Verdict=%s&NextCard=%d",
    displayName,
    activeCard,
    userChoice,
    verdict,
    nextCard,
  )
  restClient.R().EnableTrace().Post(url)
}

func main() {
  restClient := resty.New()
  session := map[string]Game{}

  client := twitch.NewAnonymousClient()

  client.OnPrivateMessage(func(rawMessage twitch.PrivateMessage) {
    displayName := rawMessage.User.DisplayName
    message := rawMessage.Message
    userId := rawMessage.User.ID

    session = HandleMessage(restClient, session, displayName, message, userId)
  })

  client.Join("AyMeeko")

  err := client.Connect()
  if err != nil {
    panic(err)
  }
}
