package chesstools

import (
	_ "embed"

	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/notnil/chess"
)

const BaseUrl = "https://explorer.lichess.ovh/lichess"

//go:embed eco/all_fen.tsv
var openingNamesTsvText string
var openingNames map[string]string

type MoveStats struct {
	Uci       string `json:"uci"`
	San       string `json:"san"`
	WhiteWins int    `json:"white"`
	BlackWins int    `json:"black"`
	Draws     int    `json:"draws"`
}

type OpeningResp struct {
	WhiteWins int         `json:"white"`
	BlackWins int         `json:"black"`
	Draws     int         `json:"draws"`
	Moves     []MoveStats `json:"moves"`
}

type OpeningGame struct {
	G           *chess.Game
	parent      *OpeningGame
	openingName string
	OpeningResp *OpeningResp
	Threshold   float64 // percent of games
}

func (openingGame *OpeningGame) String() string {
	return openingGame.openingName
}

func (openingGame *OpeningGame) Turn() chess.Color {
	return openingGame.G.Position().Turn()
}

func NewOpeningGame(parent *OpeningGame, move string, getTop bool,
	gapThreshold float64) (*OpeningGame, error) {

	var openingGame OpeningGame
	openingGame.Threshold = gapThreshold
	if parent == nil {
		openingGame.G = chess.NewGame()
	} else {
		parentMovesStr := parent.G.String()
		parentMovesReader := strings.NewReader(parentMovesStr)
		parentMoves, err := chess.PGN(parentMovesReader)
		if err != nil {
			return nil, err
		}
		openingGame.G = chess.NewGame(parentMoves)
	}
	var err error
	if move != "" {
		notation := chess.UseNotation(chess.UCINotation{})
		notation(openingGame.G)
		err := openingGame.G.MoveStr(move)
		if err != nil {
			notation := chess.UseNotation(chess.AlgebraicNotation{})
			notation(openingGame.G)
			err = openingGame.G.MoveStr(move)
		}
		if err != nil {
			return nil, err
		}
	}
	openingGame.parent = parent
	if getTop {
		openingGame.OpeningResp, err = getTopMoves(openingGame.G)
		if err != nil {
			return nil, err
		}
	}
	var ok bool
	openingGame.openingName, ok = openingNames[openingGame.G.FEN()]
	if !ok {
		if parent != nil {
			openingGame.openingName = parent.openingName
		} else {
			openingGame.openingName = ""
		}
	}

	return &openingGame, nil
}

func (openingGame *OpeningGame) ChoicesString() string {
	var sb strings.Builder

	total := openingGame.OpeningResp.Total()

	sb.WriteString(fmt.Sprintf("white:%v black:%v draws:%v\n",
		PctS(openingGame.OpeningResp.WhiteWins, total),
		PctS(openingGame.OpeningResp.BlackWins, total),
		PctS(openingGame.OpeningResp.Draws, total)))
	for _, mv := range openingGame.OpeningResp.Moves {
		tmpGame, err := NewOpeningGame(openingGame, mv.San, false,
			openingGame.Threshold)
		var gameName string
		if err != nil {
			gameName = fmt.Sprintf("err:%v", err)
		} else {
			gameName = tmpGame.String()
		}

		mvTotal := mv.Total()
		if Pct(mvTotal, total) < openingGame.Threshold {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %v(%v) white:%v black:%v draws:%v (%v)\n",
			mv.San, PctS(mvTotal, total), PctS(mv.WhiteWins, mvTotal),
			PctS(mv.BlackWins, mvTotal), PctS(mv.Draws, mvTotal), gameName))
	}

	return sb.String()
}

func (openingResp *OpeningResp) Total() int {
	return openingResp.BlackWins + openingResp.WhiteWins + openingResp.Draws
}

func (mv *MoveStats) Total() int {
	return mv.BlackWins + mv.WhiteWins + mv.Draws
}

func Pct(numerator int, denominator int) float64 {
	pctFloat := float64(numerator) / float64(denominator)
	return pctFloat
}

func PctS(numerator int, denominator int) string {
	pctInt := int(Pct(numerator, denominator) * 100.0)
	return fmt.Sprintf("%v%%", pctInt)
}

func PctS2(pctf float64) string {
	pctInt := int(pctf * 100.0)
	return fmt.Sprintf("%v%%", pctInt)
}

func getTopMoves(g *chess.Game) (*OpeningResp, error) {
	position := url.QueryEscape(g.FEN())
	ratingBuckets := url.QueryEscape("2200,2500")
	speeds := url.QueryEscape("blitz,rapid,classical")

	queryParams :=
		fmt.Sprintf("?fen=%v&ratings=%v&speeds=%v", position, ratingBuckets,
			speeds)
	requestURL, err := url.Parse(BaseUrl + queryParams)
	if err != nil {
		return nil, fmt.Errorf("gapcheck: failed to parse url:%w", err)
	}

	var openingResp OpeningResp
	var resp *http.Response

	for {
		req, err := http.NewRequest("GET", requestURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("gapcheck: failed to create request:%w", err)
		}

		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gapcheck: GET %v failed: %w",
				requestURL.String(), err)
		}
		if resp.StatusCode == 429 {
			// https://lichess.org/page/api-tips says wait a minute
			fmt.Fprintf(os.Stderr, "429 recv; sleeping 1min...")

			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
			client = nil
			resp = nil
			req = nil

			time.Sleep(1 * time.Minute)
			continue
		}

		defer resp.Body.Close()
		break
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gapcheck: failed to read http response: %w", err)
	}

	err = json.Unmarshal(body, &openingResp)
	if err != nil {
		return nil, fmt.Errorf("gapcheck: failed to unmarshel json response.\n\terr:%w\n\tcode:%v\n\tbody:%v", err, resp.StatusCode, string(body))
	}

	return &openingResp, nil

}

func init() {
	openingNames = make(map[string]string)
	for _, openingRow := range strings.Split(openingNamesTsvText, "\n") {
		openingFields := strings.Split(openingRow, ";")
		if len(openingFields) != 2 {
			continue
		}

		fen := openingFields[0]
		name := openingFields[1]
		openingNames[fen] = name
	}
}
