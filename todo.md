
(/) - allow capital letters to be mobile friendly
(/) - change the green highlight color to blue?
(/) - add an area in the game window that will allow the game to notify a user they sent a message too fast
    - have the game loop clear the notification when the user sends a message that is processed
- keep track of how many successful guesses you've had this game and display it
- keep track of each player's highest score this session and display it
- allow users to specify the range in the join message, default to 13 but allow up to 50?
- figure out how to add a game timeout so that a user who has started a game and left will eventually have their game time out and allow someone else to play


- should i be passing the Game reference instead
- ensure every time i touch a session map (main.go and game.go), we lock it and unlock it so two requests don't try to touch it at the same time
- refactor the main.go code so the structs make more sense (mirror the game.go files)
