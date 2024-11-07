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

var openingsByFEN map[string]*Opening
var openingsByNormalFEN map[string]*Opening

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
	opponent        string
	opponentColor   chess.Color
	haveTopReplies  bool
}

func (openingGame *OpeningGame) String() string {
	return openingGame.openingName
}

func (openingGame *OpeningGame) Turn() chess.Color {
	return openingGame.G.Position().Turn()
}

func NewOpeningGame() *OpeningGame {
	g := chess.NewGame()

	openingGame := &OpeningGame{
		G:               g,
		Parent:          nil,
		openingName:     "",
		Eco:             "",
		OpeningResp:     &OpeningResp{},
		Threshold:       0.9999999999,
		eval:            false,
		fullRatingRange: false,
		allSpeeds:       false,
		fromFen:         false,
		opponent:        "",
		opponentColor:   chess.NoColor,
		haveTopReplies:  false,
	}

	return openingGame
}

func (openingGame *OpeningGame) WithThreshold(threshold float64) *OpeningGame {
	openingGame.Threshold = threshold

	return openingGame
}

func (openingGame *OpeningGame) WithOpponent(playerIn string, colorIn chess.Color) *OpeningGame {
	openingGame.opponent = playerIn
	openingGame.opponentColor = colorIn

	return openingGame
}

func (openingGame *OpeningGame) WithAllSpeeds(fetchAllSpeeds bool) *OpeningGame {
	openingGame.allSpeeds = fetchAllSpeeds

	return openingGame
}

func (openingGame *OpeningGame) WithFullRatingRange(fetchFullRatingRange bool) *OpeningGame {
	openingGame.fullRatingRange = fetchFullRatingRange

	return openingGame
}

func (openingGame *OpeningGame) WithFEN(fen string) *OpeningGame {
	newGameArgs, err := chess.FEN(fen)
	if err != nil {
		panic(fmt.Sprintf("FEN invalid err:%v fen:%v", err, fen))
	}

	openingGame = openingGame.WithGame(chess.NewGame(newGameArgs))
	openingGame.fromFen = true

	return openingGame
}

func (openingGame *OpeningGame) WithParent(parent *OpeningGame) *OpeningGame {
	if !parent.fromFen {
		parentMovesStr := parent.G.String()
		parentMovesReader := strings.NewReader(parentMovesStr)
		parentMoves, err := chess.PGN(parentMovesReader)
		if err != nil {
			panic(fmt.Sprintf("Could not parse parent err:%v moveList:%v", err,
				parentMovesStr))
		}
		openingGame.G = chess.NewGame(parentMoves)

	} else {
		parentFen := parent.G.Position().XFENString()
		newGameArgs, err := chess.FEN(parentFen)
		if err != nil {
			panic(fmt.Sprintf("Could not parse parent err:%v fen:%v", err,
				parentFen))
		}
		openingGame.G = chess.NewGame(newGameArgs)
		openingGame.fromFen = true
	}
	openingGame.opponent = parent.opponent
	openingGame.opponentColor = parent.opponentColor
	openingGame.Parent = parent

	return openingGame.withECO()
}

func (openingGame *OpeningGame) WithGame(game *chess.Game) *OpeningGame {
	openingGame.G = game

	if openingGame.Parent != nil {
		panic("WithGame() and WithParent() are mutually exclusive")
	}

	return openingGame.withECO()
}

func (openingGame *OpeningGame) WithMove(move string) *OpeningGame {
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
			if move == "Kh1" {
				move = "O-O"
				notation := chess.UseNotation(chess.AlgebraicNotation{})
				notation(openingGame.G)
				err = openingGame.G.MoveStr(move)
			}
		}
		if err != nil {
			panic(fmt.Sprintf("Could not parse move:%v in %v", move,
				openingGame.G.Moves()))
		}
	}

	return openingGame.withECO()
}

func (openingGame *OpeningGame) WithTopReplies(fetchTop bool) *OpeningGame {
	if !fetchTop {
		return openingGame
	}

	var err error
	openingGame.OpeningResp, err = getTopReplies(openingGame.G,
		openingGame.fullRatingRange, openingGame.allSpeeds, openingGame.opponent,
		openingGame.opponentColor)
	if err != nil {
		panic(fmt.Sprintf("Could not fetch top moves err:%v in %v", err,
			openingGame.String()))
	}
	openingGame.haveTopReplies = true

	return openingGame
}

func (openingGame *OpeningGame) withECO() *OpeningGame {
	var ok bool

	fen := openingGame.G.Position().XFENString()
	opening, ok := openingsByFEN[fen]
	if !ok {
		fen, err := NormalizeFEN(fen)
		if err == nil {
			opening, ok = openingsByNormalFEN[fen]
		}
	}
	if !ok {
		if openingGame.Parent != nil {
			openingGame.openingName = openingGame.Parent.openingName
			openingGame.Eco = openingGame.Parent.Eco
		} else {
			openingGame.openingName = ""
			openingGame.Eco = ""
		}
	} else {
		openingGame.openingName = opening.name
		openingGame.Eco = opening.eco
	}

	return openingGame
}

func (openingGame *OpeningGame) WithEval(doEval bool) *OpeningGame {
	openingGame.eval = doEval
	if doEval {
		err := openingGame.getEvalsForResp()
		if err != nil {
			panic(fmt.Sprintf("Could not fetch evals err:%v in %v", err,
				openingGame.String()))
		}
	}

	return openingGame
}

func (openingGame *OpeningGame) ChoicesString(ignoreThreshold bool) string {
	var sb strings.Builder

	total := openingGame.OpeningResp.Total()

	for idx, mv := range openingGame.OpeningResp.Moves {
		tmpGame := NewOpeningGame().WithParent(openingGame).WithMove(mv.San).WithThreshold(openingGame.Threshold)
		gameName := tmpGame.String()

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
				sb.WriteString(fmt.Sprintf("Mate:%v [", mv.Eval.Mate))
			} else {
				sb.WriteString(fmt.Sprintf("%v [", mv.Eval.CP))
			}
			if mv.Eval.WinPct != 0.0 || mv.Eval.DrawPct != 0.0 ||
				mv.Eval.LossPct != 0.0 {
				sb.WriteString(fmt.Sprintf("wdl:%v%%/%v%%/%v%% ",
					uint(mv.Eval.WinPct*100), uint(mv.Eval.DrawPct*100),
					uint(mv.Eval.LossPct*100)))
			}
			if mv.Eval.SearchTimeInSeconds != 0.0 {
				sb.WriteString(fmt.Sprintf("time:%vs ",
					uint(mv.Eval.SearchTimeInSeconds)))
			} else if mv.Eval.Depth != 0 {
				sb.WriteString(fmt.Sprintf("depth:%v ", mv.Eval.Depth))
			}
			sb.WriteString(fmt.Sprintf("type:%v]\n", mv.Eval.Type))
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

func getTopReplies(g *chess.Game, fullRatingRange bool,
	allSpeeds bool, opponent string, opponentColor chess.Color) (*OpeningResp, error) {

	fen := g.Position().XFENString()
	position := url.QueryEscape(fen)
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

	if opponent == "" || opponentColor != g.Position().Turn() {
		queryParams =
			fmt.Sprintf("?fen=%v&ratings=%v&speeds=%v", position, ratingBuckets,
				speeds)
		requestURL, err = url.Parse(LichessDbBaseUrl + queryParams)
	} else {
		queryParams =
			fmt.Sprintf("?player=%v&fen=%v&ratings=%v&speeds=%v&color=%v", opponent,
				position, ratingBuckets, speeds, strings.ToLower(opponentColor.Name()))
		requestURL, err = url.Parse(PlayerDbBaseUrl + queryParams)
	}
	if err != nil {
		return nil, fmt.Errorf("opening: failed to parse url:%w", err)
	}

	//fmt.Fprintf(os.Stderr, "Getting top replies for %v url:%v....\n", g.Moves(),
	//	requestURL)

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
			fmt.Fprintf(os.Stderr, "opening: 429 recv; sleeping 1min fen:%v retry:%v...\n",
				fen, retryCount)
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
	openingsByFEN = make(map[string]*Opening)
	openingsByNormalFEN = make(map[string]*Opening)
	for _, openingRow := range strings.Split(openingNamesTsvText, "\n") {
		openingFields := strings.Split(openingRow, ";")
		if len(openingFields) != 3 {
			continue
		}

		fen := openingFields[0]
		ecoIn := openingFields[1]
		nameIn := openingFields[2]
		openingsByFEN[fen] = &Opening{
			eco:  ecoIn,
			name: nameIn,
		}
		normalizedFen, err := NormalizeFEN(fen)
		if err == nil {
			openingsByNormalFEN[normalizedFen] = &Opening{
				eco:  ecoIn,
				name: nameIn,
			}
		}
	}
}

func (openingGame *OpeningGame) getEvalsForResp() error {
	if !openingGame.haveTopReplies {
		return fmt.Errorf("bug: caller must call WithTopReplies() prior to WithEval()")
	}
	evalCtx := NewEvalCtx(true)
	defer evalCtx.Close()
	evalCtx.InitEngine()

	for idx, mv := range openingGame.OpeningResp.Moves {
		tmpGame := NewOpeningGame().WithParent(openingGame).WithMove(mv.San).WithThreshold(openingGame.Threshold)
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
	opening, ok := openingsByFEN[fen]
	if !ok {
		fen, err := NormalizeFEN(fen)
		if err == nil {
			opening, ok = openingsByNormalFEN[fen]
		}
	}
	if !ok {
		return ""
	}

	return opening.name
}

func GetOpeningEco(fen string) string {
	opening, ok := openingsByFEN[fen]
	if !ok {
		fen, err := NormalizeFEN(fen)
		if err == nil {
			opening, ok = openingsByNormalFEN[fen]
		}
	}
	if !ok {
		return ""
	}

	return opening.eco
}
