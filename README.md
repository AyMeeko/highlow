## Windows
```bash
$ eval "$(ssh-agent -s)"
$ ssh-add ~/.ssh/id_ed25519_aymeeko
```

To run the Echo server,
```bash
cd ~/highlow
go run cmd/main.go
```

To run the game itself,
```bash
cd ~/highlow/cmd/highlow
go run .
```
