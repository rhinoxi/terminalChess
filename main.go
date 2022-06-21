package main

import (
	"bytes"
	b64 "encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rhinoxi/chess"
	chessImage "github.com/rhinoxi/chess/image"
	"github.com/rhinoxi/chess/uci"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const (
	PAYLOAD = "payload"
)

func serializeGrCommand(cmdM map[string]interface{}) (string, error) {
	var payload []byte
	var cmds []string
	var sb strings.Builder
	for k, v := range cmdM {
		if k == PAYLOAD {
			payload = v.([]byte)
		} else {
			cmds = append(cmds, fmt.Sprintf("%s=%v", k, v))
		}
	}

	if _, err := sb.WriteString("\033_G"); err != nil {
		return "", err
	}
	if _, err := sb.WriteString(strings.Join(cmds, ",")); err != nil {
		return "", err
	}
	if len(payload) > 0 {
		sb.WriteString(";")
		if _, err := sb.Write(payload); err != nil {
			return "", err
		}
	}
	if _, err := sb.WriteString("\033\\"); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func svg2png(r io.Reader, w io.Writer) error {
	icon, err := oksvg.ReadIconStream(r)
	if err != nil {
		return err
	}
	width, height := int(icon.ViewBox.W), int(icon.ViewBox.H)
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	icon.Draw(rasterx.NewDasher(width, height, rasterx.NewScannerGV(width, height, rgba, rgba.Bounds())), 1)

	if err := png.Encode(w, rgba); err != nil {
		return err
	}
	return nil
}

func writeChunked(data []byte) error {
	for len(data) > 0 {
		var chunk []byte
		idx := min(4096, len(data))
		chunk, data = data[:idx], data[idx:]
		m := 1
		if len(data) == 0 {
			m = 0
		}
		cmdM := map[string]interface{}{
			"m":     m,
			PAYLOAD: chunk,
			"a":     "T",
			"f":     100,
		}
		a, err := serializeGrCommand(cmdM)
		if err != nil {
			return err
		}
		os.Stdout.WriteString(a)
	}
	os.Stdout.WriteString("\n")
	return nil
}

func draw(board *chess.Board) error {
	var svg bytes.Buffer
	if err := chessImage.SVG(&svg, board); err != nil {
		return err
	}
	var bf bytes.Buffer
	if err := svg2png(&svg, &bf); err != nil {
		return err
	}

	dataIn := bf.Bytes()

	data := make([]byte, b64.StdEncoding.EncodedLen(len(dataIn)))

	b64.StdEncoding.Encode(data, dataIn)

	if err := writeChunked(data); err != nil {
		return err
	}
	return nil
}

func clearScreen() {
	fmt.Print("\x1bc")
}

func startGame(eng *uci.Engine) {
	game := chess.NewGame()
	var moveStr string
	for game.Outcome() == chess.NoOutcome {
		clearScreen()
		if err := draw(game.Position().Board()); err != nil {
			panic(err)
		}
		for {
			fmt.Scanf("%s", &moveStr)
			if err := game.MoveStr(moveStr); err != nil {
				fmt.Print("\x1b[1A\x1b[2K")
				fmt.Print("invalid move, try again:")
				continue
			}
			break
		}

		cmdPos := uci.CmdPosition{Position: game.Position()}
		cmdGo := uci.CmdGo{MoveTime: time.Second / 100}
		if err := eng.Run(cmdPos, cmdGo); err != nil {
			panic(err)
		}
		move := eng.SearchResults().BestMove
		if err := game.Move(move); err != nil {
			panic(err)
		}

		game.Move(move)
	}

	clearScreen()
	if err := draw(game.Position().Board()); err != nil {
		panic(err)
	}
	fmt.Printf("Game completed. %s by %s.\n", game.Outcome(), game.Method())
	fmt.Println(game.String())
}

func main() {
	eng, err := uci.New("stockfish")
	if err != nil {
		panic(err)
	}
	defer eng.Close()

	if err := eng.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		panic(err)
	}

	startGame(eng)
}
