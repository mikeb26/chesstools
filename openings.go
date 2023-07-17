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

const (
	LichessDbBaseUrl = "https://explorer.lichess.ovh/lichess"
	PlayerDbBaseUrl  = "https://explorer.lichess.ovh/player"
)

//go:embed eco/all_fen.tsv
var openingNamesTsvText string

type Opening struct {
	eco  string
	name string
}

var openings map[string]*Opening

type MoveStats struct {
	Uci       string `json:"uci"`
	San       string `json:"san"`
	WhiteWins int    `json:"white"`
	BlackWins int    `json:"black"`
	Draws     int    `json:"draws"`

	Eval *EvalResult //only valid when getEval==true
}

type PlayerInfo struct {
	Name   string `json:"name"`
	Rating int    `json:"rating"`
}

type GameInfo struct {
	Uci    string     `json:"uci"`
	Id     string     `json:"id"`
	Winner string     `json:"winner"`
	Speed  string     `json:"speed"`
	Mode   string     `json:"mode"`
	Year   int        `json:"year"`
	Month  string     `json:"month"`
	Black  PlayerInfo `json:"black"`
	White  PlayerInfo `json:"white"`
}

type OpeningResp struct {
	WhiteWins   int         `json:"white"`
	BlackWins   int         `json:"black"`
	Draws       int         `json:"draws"`
	Moves       []MoveStats `json:"moves"`
	TopGames    []GameInfo  `json:"topGames"`
	RecentGames []GameInfo  `json:"recentGames"`
}

type OpeningGame struct {
	G           *chess.Game
	Parent      *OpeningGame
	openingName string
	Eco         string
	OpeningResp *OpeningResp
	Threshold   float64 // percent of games

	eval            bool
	fullRatingRange bool
	allSpeeds       bool
	fromFen         bool
	player          string
	color           string
}

func (openingGame *OpeningGame) String() string {
	return openingGame.openingName
}

func (openingGame *OpeningGame) Turn() chess.Color {
	return openingGame.G.Position().Turn()
}

func NewOpeningGame(parent *OpeningGame, move string, getTop bool,
	threshold float64, getEval bool) (*OpeningGame, error) {
	return NewOpeningGameActual(parent, nil, move, getTop, threshold, getEval,
		false, false, "", "")
}

func NewOpeningGame2(game *chess.Game, getTop bool,
	threshold float64, getEval bool) (*OpeningGame, error) {
	return NewOpeningGameActual(nil, game, "", getTop, threshold, getEval,
		false, false, "", "")
}

func NewOpeningGame3(fen string, playerIn string,
	colorIn string) (*OpeningGame, error) {

	newGameArgs, err := chess.FEN(fen)
	if err != nil {
		return nil, err
	}

	g := chess.NewGame(newGameArgs)
	openingGame, err := NewOpeningGameActual(nil, g, "", true, 0.999999999,
		false, true, true, playerIn, colorIn)
	if err != nil {
		return nil, err
	}

	openingGame.fromFen = true

	return openingGame, nil
}

func NewOpeningGame4(parent *OpeningGame, move string) (*OpeningGame, error) {
	return NewOpeningGameActual(parent, nil, move, true, 0.999999999, false,
		true, true, "", "")
}

func NewOpeningGameActual(parent *OpeningGame, game *chess.Game, move string,
	getTop bool, threshold float64, getEval bool,
	fullRatingRange bool, allSpeeds bool, playerIn string,
	colorIn string) (*OpeningGame, error) {

	var openingGame OpeningGame
	openingGame.fromFen = false
	openingGame.Threshold = threshold
	openingGame.player = playerIn
	openingGame.color = colorIn

	if parent == nil {
		if game == nil {
			openingGame.G = chess.NewGame()
		} else {
			openingGame.G = game
		}
	} else {
		if !parent.fromFen {
			parentMovesStr := parent.G.String()
			parentMovesReader := strings.NewReader(parentMovesStr)
			parentMoves, err := chess.PGN(parentMovesReader)
			if err != nil {
				return nil, err
			}
			openingGame.G = chess.NewGame(parentMoves)
		} else {
			parentFen := parent.G.Position().XFENString()
			newGameArgs, err := chess.FEN(parentFen)
			if err != nil {
				return nil, err
			}
			openingGame.G = chess.NewGame(newGameArgs)
			openingGame.fromFen = true
		}
		openingGame.player = parent.player
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
	openingGame.Parent = parent
	if getTop {
		openingGame.OpeningResp, err = getTopMoves(openingGame.G,
			fullRatingRange, allSpeeds, openingGame.player, openingGame.color)
		if err != nil {
			return nil, err
		}
	}
	var ok bool

	opening, ok := openings[openingGame.G.Position().XFENString()]
	if !ok {
		if parent != nil {
			openingGame.openingName = parent.openingName
			openingGame.Eco = parent.Eco
		} else {
			openingGame.openingName = ""
			openingGame.Eco = ""
		}
	} else {
		openingGame.openingName = opening.name
		openingGame.Eco = opening.eco
	}
	openingGame.eval = getEval
	openingGame.fullRatingRange = fullRatingRange
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
		sb.WriteString(fmt.Sprintf("  %v. %v (%v) [%v]\n", idx+1, mv.San,
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

func getTopMoves(g *chess.Game, fullRatingRange bool,
	allSpeeds bool, player string, color string) (*OpeningResp, error) {

	position := url.QueryEscape(g.FEN())
	var ratingBuckets string
	if fullRatingRange {
		ratingBuckets =
			url.QueryEscape("400,1000,1200,1400,1600,1800,2000,2200,2500")
	} else {
		ratingBuckets = url.QueryEscape("2200,2500")
	}
	var speeds string
	if allSpeeds {
		speeds = url.QueryEscape("ultraBullet,bullet,blitz,rapid,classical,correspondence")
	} else {
		speeds = url.QueryEscape("blitz,rapid,classical")
	}

	var queryParams string
	var requestURL *url.URL
	var err error

	if player == "" {
		queryParams =
			fmt.Sprintf("?fen=%v&ratings=%v&speeds=%v", position, ratingBuckets,
				speeds)
		requestURL, err = url.Parse(LichessDbBaseUrl + queryParams)
	} else {
		queryParams =
			fmt.Sprintf("?player=%v&fen=%v&ratings=%v&speeds=%v&color=%v", player,
				position, ratingBuckets, speeds, color)
		requestURL, err = url.Parse(PlayerDbBaseUrl + queryParams)
	}
	if err != nil {
		return nil, fmt.Errorf("opening: failed to parse url:%w", err)
	}

	var openingResp OpeningResp
	var resp *http.Response
	retryCount := 0

	for {
		req, err := http.NewRequest("GET", requestURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("opening: failed to create request:%w", err)
		}

		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("opening: GET %v failed: %w",
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

	// can't use json.Unmarshal() because results may be newline delimited
	// JSON
	decoder := json.NewDecoder(resp.Body)
	decodedOne := false
	for decoder.More() { // just use the last result
		if decodedOne {
			fmt.Fprintf(os.Stderr, ".")
		}
		err := decoder.Decode(&openingResp)
		if err != nil {
			return nil, fmt.Errorf("opening: failed to unmarshal json response.\n\turl:%v\n\terr:%w\n\tcode:%v\n",
				requestURL.String(), err, resp.StatusCode)
		}
		decodedOne = true
	}

	if openingResp.TopGames == nil || len(openingResp.TopGames) == 0 {
		openingResp.TopGames = openingResp.RecentGames
		openingResp.RecentGames = make([]GameInfo, 0)
	}

	return &openingResp, nil

}

func init() {
	openings = make(map[string]*Opening)
	for _, openingRow := range strings.Split(openingNamesTsvText, "\n") {
		openingFields := strings.Split(openingRow, ";")
		if len(openingFields) != 3 {
			continue
		}

		fen := openingFields[0]
		ecoIn := openingFields[1]
		nameIn := openingFields[2]
		openings[fen] = &Opening{
			eco:  ecoIn,
			name: nameIn,
		}
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
	opening, ok := openings[fen]
	if !ok {
		return ""
	}

	return opening.name
}

func GetOpeningEco(fen string) string {
	opening, ok := openings[fen]
	if !ok {
		return ""
	}

	return opening.eco
}
