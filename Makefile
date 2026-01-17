
randrange: main.go
	go build -ldflags="-s -w" -o randrange

clean:
	command rm -f randrange .??*~
