package chesstools

import (
	_ "embed"
	"strconv"

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

	Eval *EvalResult //only valid when getEval==true
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

	eval bool
}

func (openingGame *OpeningGame) String() string {
	return openingGame.openingName
}

func (openingGame *OpeningGame) Turn() chess.Color {
	return openingGame.G.Position().Turn()
}

func NewOpeningGame(parent *OpeningGame, move string, getTop bool,
	threshold float64, getEval bool) (*OpeningGame, error) {
	return NewOpeningGameActual(parent, nil, move, getTop, threshold, getEval)
}

func NewOpeningGame2(game *chess.Game, getTop bool,
	threshold float64, getEval bool) (*OpeningGame, error) {
	return NewOpeningGameActual(nil, game, "", getTop, threshold, getEval)
}

func NewOpeningGameActual(parent *OpeningGame, game *chess.Game, move string,
	getTop bool, threshold float64, getEval bool) (*OpeningGame, error) {

	var openingGame OpeningGame
	openingGame.Threshold = threshold
	if parent == nil {
		if game == nil {
			openingGame.G = chess.NewGame()
		} else {
			openingGame.G = game
		}
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
	openingGame.openingName, ok = openingNames[openingGame.G.Position().XFENString()]
	if !ok {
		if parent != nil {
			openingGame.openingName = parent.openingName
		} else {
			openingGame.openingName = ""
		}
	}
	openingGame.eval = getEval
	if getEval {
		err = openingGame.getEvalsForResp()
		if err != nil {
			return nil, err
		}
	}

	return &openingGame, nil
}

func (openingGame *OpeningGame) ChoicesString(ignoreThreshold bool) string {
	var sb strings.Builder

	total := openingGame.OpeningResp.Total()

	for idx, mv := range openingGame.OpeningResp.Moves {
		tmpGame, err := NewOpeningGame(openingGame, mv.San, false,
			openingGame.Threshold, false)
		var gameName string
		if err != nil {
			gameName = fmt.Sprintf("err:%v", err)
		} else {
			gameName = tmpGame.String()
		}

		mvTotal := mv.Total()
		if !ignoreThreshold && Pct(mvTotal, total) < openingGame.Threshold {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %v. %v (%v) [%v]\n", idx, mv.San,
			PctS(mvTotal, total), gameName))
		sb.WriteString(fmt.Sprintf("      Winning Percentages: ["))
		sb.WriteString(fmt.Sprintf("White:%v ", PctS(mv.WhiteWins, mvTotal)))
		sb.WriteString(fmt.Sprintf("Black:%v ", PctS(mv.BlackWins, mvTotal)))
		sb.WriteString(fmt.Sprintf("Draws:%v]\n", PctS(mv.Draws, mvTotal)))

		if openingGame.eval && mv.Eval != nil {
			sb.WriteString(fmt.Sprintf("      Eval: "))
			if mv.Eval.Mate != 0 {
				sb.WriteString(fmt.Sprintf("Mate:%v ", mv.Eval.Mate))
			} else {
				sb.WriteString(fmt.Sprintf("%v ", mv.Eval.CP))
			}
			sb.WriteString(fmt.Sprintf("(depth:%v)\n", mv.Eval.Depth))
		}
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
	retryCount := 0

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
			fmt.Fprintf(os.Stderr, "opening: 429 recv; sleeping 1min retry:%v...",
				retryCount)
			retryCount++

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

func (openingGame *OpeningGame) getEvalsForResp() error {
	evalCtx := NewEvalCtx(true)
	defer evalCtx.Close()

	for idx, mv := range openingGame.OpeningResp.Moves {
		tmpGame, err := NewOpeningGame(openingGame, mv.San, false,
			openingGame.Threshold, false)
		if err != nil {
			return err
		}
		evalCtx.SetFEN(tmpGame.G.FEN())
		openingGame.OpeningResp.Moves[idx].Eval = evalCtx.Eval()
	}

	return nil
}

func getMoveCountFromFEN(fen string) int {
	// move count is encoded as the last token in the fen
	fenTokens := strings.Split(fen, " ")
	lastTokenIdx := len(fenTokens) - 1

	ret, _ := strconv.ParseInt(fenTokens[lastTokenIdx], 10, 32)
	return int(ret)
}

func (openingGame *OpeningGame) GetMoveCount() int {
	return getMoveCountFromFEN(openingGame.G.Position().XFENString())
}

func GetOpeningName(fen string) string {
	openingName, ok := openingNames[fen]
	if !ok {
		return ""
	}

	return openingName
}
