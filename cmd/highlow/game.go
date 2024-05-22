/*
$ go run ./cmd/highlow/game.go
*/
package main

import (
  "fmt"
  "math/rand"

  "github.com/gempir/go-twitch-irc/v4"
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

func HandleMessage(session map[string]Game, displayName, message, userId string) map[string]Game {
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
  } else {
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
        } else {
          fmt.Printf("Correct! Active card: %d\n", nextCard)
        }
      } else {
        fmt.Printf("Incorrect! The next card was %d. Better luck next time!\n", nextCard)
        delete(session, displayName)
      }
    }
  }
  return session
}

func main() {
  session := map[string]Game{}

  client := twitch.NewAnonymousClient()

  client.OnPrivateMessage(func(rawMessage twitch.PrivateMessage) {
    displayName := rawMessage.User.DisplayName
    message := rawMessage.Message
    userId := rawMessage.User.ID

    session = HandleMessage(session, displayName, message, userId)
  })

  client.Join("AyMeeko")

  err := client.Connect()
  if err != nil {
    panic(err)
  }
}
