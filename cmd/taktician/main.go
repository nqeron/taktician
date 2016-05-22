package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/nelhage/taktician/ai"
	"github.com/nelhage/taktician/playtak"
	"github.com/nelhage/taktician/ptn"
	"github.com/nelhage/taktician/tak"
)

var (
	server   = flag.String("server", "playtak.com:10000", "playtak.com server to connect to")
	user     = flag.String("user", "", "username for login")
	pass     = flag.String("pass", "", "password for login")
	accept   = flag.String("accept", "", "accept a game from specified user")
	gameTime = flag.Duration("time", 20*time.Minute, "Length of game to offer")
	size     = flag.Int("size", 5, "size of game to offer")
	once     = flag.Bool("once", false, "play a single game and exit")
	takbot   = flag.String("takbot", "", "challenge TakBot AI")

	debug = flag.Int("debug", 1, "debug level")
	depth = flag.Int("depth", 5, "minimax depth")
	limit = flag.Duration("limit", time.Minute, "time limit per move")
	sort  = flag.Bool("sort", true, "sort moves via history heuristic")
	table = flag.Bool("table", true, "use the transposition table")

	debugClient = flag.Bool("debug-client", false, "log debug output for playtak connection")
)

const ClientName = "Taktician AI"

func main() {
	flag.Parse()
	if *accept != "" || *takbot != "" {
		*once = true
	}

	backoff := 1 * time.Second
	for {
		client := &playtak.Client{
			Debug: *debugClient,
		}
		err := client.Connect(*server)
		if err != nil {
			goto reconnect
		}
		backoff = time.Second
		client.SendClient(ClientName)
		if *user != "" {
			err = client.Login(*user, *pass)
		} else {
			err = client.LoginGuest()
		}
		if err != nil {
			log.Fatal("login: ", err)
		}
		log.Printf("login OK")
		for {
			if *accept != "" {
				for line := range client.Recv {
					if strings.HasPrefix(line, "Seek new") {
						bits := strings.Split(line, " ")
						if bits[3] == *accept {
							log.Printf("accepting game %s from %s", bits[2], bits[3])
							client.SendCommand("Accept", bits[2])
							break
						}
					}
				}
			} else {
				client.SendCommand("Seek", strconv.Itoa(*size), strconv.Itoa(int(gameTime.Seconds())))
				log.Printf("Seek OK")
				if *takbot != "" {
					client.SendCommand("Shout", "takbot: play", *takbot)
				}
			}
			for line := range client.Recv {
				if strings.HasPrefix(line, "Game Start") {
					playGame(client, line)
					break
				}
			}
			if *once {
				return
			}
			if client.Error() != nil {
				log.Printf("Disconnected: %v", client.Error())
				break
			}
		}
	reconnect:
		log.Printf("sleeping %s before reconnect...", backoff)
		time.Sleep(backoff)
		backoff = backoff * 2
		if backoff > time.Minute {
			backoff = time.Minute
		}
	}
}

func timeBound(remaining time.Duration) time.Duration {
	return *limit
}

func playGame(c *playtak.Client, line string) {
	bits := strings.Split(line, " ")
	size, _ := strconv.Atoi(bits[3])
	ai := ai.NewMinimax(ai.MinimaxConfig{
		Size:  size,
		Depth: *depth,
		Debug: *debug,

		NoSort:  !*sort,
		NoTable: !*table,
	})
	p := tak.New(tak.Config{Size: size})
	id := bits[2]
	gameStr := fmt.Sprintf("Game#%s", id)
	var color tak.Color
	switch bits[7] {
	case "white":
		color = tak.White
	case "black":
		color = tak.Black
	default:
		panic(fmt.Sprintf("bad color: %s", bits[7]))
	}
	log.Printf("new game game-id=%q size=%s player1=%q player2=%q color=%q time=%q",
		bits[2], bits[3], bits[4], bits[6], bits[7], bits[8],
	)
	timeLeft := *gameTime
	for {
		over, _ := p.GameOver()
		if color == p.ToMove() && !over {
			move := ai.GetMove(p, timeBound(timeLeft))
			next, err := p.Move(&move)
			if err != nil {
				log.Printf("ai returned bad move: %s: %s",
					ptn.FormatMove(&move), err)
				continue
			}
			p = next
			c.SendCommand(gameStr, playtak.FormatServer(&move))
			log.Printf("my-move game-id=%s ply=%d ptn=%d.%s move=%q",
				id,
				p.MoveNumber(),
				p.MoveNumber()/2+1,
				strings.ToUpper(color.String()[:1]),
				ptn.FormatMove(&move))
		} else {
			var timeout <-chan time.Time
		theirMove:
			for {
				var line string
				select {
				case line = <-c.Recv:
				case <-timeout:
					break theirMove
				}

				if !strings.HasPrefix(line, gameStr) {
					continue
				}
				bits = strings.Split(line, " ")
				switch bits[1] {
				case "P", "M":
					move, err := playtak.ParseServer(strings.Join(bits[1:], " "))
					if err != nil {
						panic(err)
					}
					p, err = p.Move(&move)
					if err != nil {
						panic(err)
					}
					log.Printf("their-move game-id=%s ply=%d ptn=%d%s move=%q",
						id,
						p.MoveNumber(),
						p.MoveNumber()/2+1,
						strings.ToUpper(color.Flip().String()[:1]),
						ptn.FormatMove(&move))
					timeout = time.NewTimer(500 * time.Millisecond).C
				case "Abandoned.":
					log.Printf("game-over game-id=%s ply=%d result=abandoned",
						id, p.MoveNumber())
					return
				case "Over":
					log.Printf("game-over game-id=%s ply=%d result=%q",
						id, p.MoveNumber(), bits[2])
					return
				case "Time":
					w, b := bits[2], bits[3]
					var secsLeft int
					if color == tak.White {
						secsLeft, _ = strconv.Atoi(w)
					} else {
						secsLeft, _ = strconv.Atoi(b)
					}
					timeLeft = time.Duration(secsLeft) * time.Second
					break theirMove
				}
			}
		}
	}
}
