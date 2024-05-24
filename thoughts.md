## Long term upgrades
- add a timeout so that if someone starts a game and never finishes it, the game cleans up and allows
    someone else to play

- In terms of design, I'd like each game to be self contained in a square.
- The title of the square will be the player's name
- when there is only one player, the square will be in the middle of the screen
- when there are two players, the squares will be next to each other and centered
- when there are three players, the squares will move into a 2x2 grid with a placeholder game square
- when there are four players, the games will show in a 2x2 grid.

- when a user sends "h" or "l", the label indicating that option in their game should highlight a pending color
- after a second, if it was correct, the color should change to green and show the text "Correct!"


- set up an http server in go that accepts various verbs
    - POST newGame
        - body: displayName, userId
    -


even if i set up the htmx elements to poll the Go server every 2s, how does the Go server
know what the game state is?

I can't just put the game logic into the Go server because the irc client connection seems to
not work when i do that.

so.. if the front end htmx and echo server are disjointed from the game loop, that means:
- the game loop needs to make API requests to the echo server to update the FE
- the FE needs to have its own concept of the game components

# Overall
body
    Game rules / how to play text
    list of games (1-4)
        DisplayName
        ActiveCard
        Lower or Higher "buttons"


# One game, waiting for user input
body
    Game rules / how to play text
    1 game:
        DisplayName
        ActiveCard
        Lower or Higher "buttons"

#  One game, reacting to user input
body
    Game rules / how to play text
    1 game:
        DisplayName
        ActiveCard
        Lower or Higher "buttons" => highlight their selection and then replace with result
# One game, user wins
body
    Game rules / how to play text
    1 game:
        DisplayName
        "You win" text => fade out after 10 seconds



- spin up the Echo server and it would show a blank index
- spin up the HighLow game and it would hit `POST /new-session` to init a new session
- player would `!j` join, and HighLow game would hit `POST /new-game` to init a new game for user
    - request body would contain DisplayName, ActiveCard
- Echo server would reflect the first game in the FE
- player would `h`, and HighLow game would hit `PUT /game` to update the game state
    - request body would contain DisplayName, new ActiveCard, and user choice
- when player wins, HighLow game would hit `PUT /winner`
    - request body would contain DisplayName and user choice
- when player loses, HighLow game would hit `PUT /loser`
    - request body would contain DisplayName and user choice



I'm thinking that when I first load the page with no data, it creates placeholders for each game
that are already refeshing every 2s. Then, when I make the request to create a new game, it will
choose one of those placeholders and insert the game there.


A few thoughts:
- I could have the Go server initialize n empty games for it render
- I could have the html file hard code how many empty games to render


If the FE connects to the Go server via websockets, I can use hx-ws to keep fetching from the



The browser will `GET /game` every 2 seconds
Game states: not_started, in_progress, won, lost
that endpoint needs to understand:
- if game.State == "not_started" => render "game" with placeholder game
- if game.State == "in_progress" => render "game"
- if game.State == "won" => render "result" with win text
- if game.State == "lost" => render "result" with lost text

the "game" block pings "GET /game" every 2 seconds
the "result" block pings "initialize-placeholder" every 10 seconds
the "initialize-placeholder" endpoint will render "game"


- `POST new-game` => display name and active card
=> starts the game for the user with the active card
- `POST game` => display name, active card (same), user choice, verdict
- needs to render the game with the choice highlighted
- needs to render whether the choice was correct
- needs to render the 'lost' OR new number


maybe there's just one channel and i have a new mapping of DisplayName: bool that indicates
whether they're rate limited

- program starts, initialize session, channel, and rateLimiter mapping (default false)
- inside the OnPrivateMessage function,
    - if !rateLimiter[displayName] { handleMessage }





## main.go refactor
- index loads all the placeholders
- the placeholder html is distinct from the game html
- the placeholder html pings the server and
    if there's a new game to slot, it renders the "game" block in its place
        else renders placeholder again
- when a game ends, render placeholder html again

when the placeholder makes the request to the server, how does the server know there is a new game to render?
