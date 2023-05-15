package main

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

type RepGame struct {
	g           *chess.Game
	parent      *RepGame
	openingName string
	openingResp *OpeningResp
	threshold   float64 // percent of games
}

func (repGame *RepGame) String() string {
	return repGame.openingName
}

func (repGame *RepGame) Turn() chess.Color {
	return repGame.g.Position().Turn()
}

func NewRepGame(parent *RepGame, move string, getTop bool,
	gapThreshold float64) (*RepGame, error) {

	var repGame RepGame
	repGame.threshold = gapThreshold
	if parent == nil {
		repGame.g = chess.NewGame()
	} else {
		parentMovesStr := parent.g.String()
		parentMovesReader := strings.NewReader(parentMovesStr)
		parentMoves, err := chess.PGN(parentMovesReader)
		if err != nil {
			return nil, err
		}
		repGame.g = chess.NewGame(parentMoves)
	}
	var err error
	if move != "" {
		notation := chess.UseNotation(chess.UCINotation{})
		notation(repGame.g)
		err := repGame.g.MoveStr(move)
		if err != nil {
			notation := chess.UseNotation(chess.AlgebraicNotation{})
			notation(repGame.g)
			err = repGame.g.MoveStr(move)
		}
		if err != nil {
			return nil, err
		}
	}
	repGame.parent = parent
	if getTop {
		repGame.openingResp, err = getTopMoves(repGame.g)
		if err != nil {
			return nil, err
		}
	}
	var ok bool
	repGame.openingName, ok = openingNames[repGame.g.FEN()]
	if !ok {
		if parent != nil {
			repGame.openingName = parent.openingName
		} else {
			repGame.openingName = ""
		}
	}

	return &repGame, nil
}

func (repGame *RepGame) ChoicesString() string {
	var sb strings.Builder

	total := repGame.openingResp.Total()

	sb.WriteString(fmt.Sprintf("white:%v black:%v draws:%v\n",
		PctS(repGame.openingResp.WhiteWins, total), PctS(repGame.openingResp.BlackWins, total),
		PctS(repGame.openingResp.Draws, total)))
	for _, mv := range repGame.openingResp.Moves {
		tmpGame, err := NewRepGame(repGame, mv.San, false, repGame.threshold)
		var gameName string
		if err != nil {
			gameName = fmt.Sprintf("err:%v", err)
		} else {
			gameName = tmpGame.String()
		}

		mvTotal := mv.Total()
		if Pct(mvTotal, total) < repGame.threshold {
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
		fmt.Sprintf("?fen=%v&ratings=%v&speeds=%v", position, ratingBuckets, speeds)
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

func initOpeningNames() {
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

func (rv *RepValidator) selectMove(repGame *RepGame, totalPct float64) (string, error) {
	normalizedFen, err := normalizeFEN(repGame.g.FEN())
	if err != nil {
		return "", fmt.Errorf("Failed to normalize FEN %v: %w", repGame.g.FEN(), err)
	}

	moveMapVal, ok := rv.moveMap[normalizedFen]
	if !ok {
		return "", nil
	}

	return moveMapVal.move, nil
}

func (rv *RepValidator) buildRep(repGame *RepGame, color chess.Color,
	totalPct float64, gapSkip int, stackDepth int) (bool, error) {

	if repGame.Turn() == color {
		mv, err := rv.selectMove(repGame, totalPct)
		if err != nil {
			return false, err
		}
		if mv == "" {
			if gapSkip == 0 && totalPct < 0.999 {
				fmt.Printf("  gap:%v(%v) pct:%v\n", repGame.g.String(),
					repGame.String(), PctS2(totalPct))
			}
			return false, nil
		}
		childGame, err := NewRepGame(repGame, mv, true, repGame.threshold)
		if err != nil {
			return false, err
		}
		return rv.buildRep(childGame, color, totalPct, gapSkip, stackDepth+1)
	} // else

	pushedOne := false
	total := repGame.openingResp.Total()
	for _, mv := range repGame.openingResp.Moves {
		mvTotal := mv.Total()
		if Pct(mvTotal, total)*totalPct < rv.gapThreshold {
			continue
		}
		pushedOne = true

		childGame, err := NewRepGame(repGame, mv.San, true, repGame.threshold)
		if err != nil {
			return false, err
		}
		var childTotalPct float64
		var childGapSkip int
		childGapSkip = gapSkip
		if childGapSkip > 0 {
			childGapSkip--
			childTotalPct = totalPct
		} else {
			childTotalPct = totalPct * Pct(mvTotal, total)
		}
		_, err = rv.buildRep(childGame, color, childTotalPct, childGapSkip,
			stackDepth+1)
		if err != nil {
			return false, err
		}
	}

	return pushedOne, nil
}

func (rv *RepValidator) checkForGaps() error {
	initOpeningNames()

	repGame, err := NewRepGame(nil, "", true, rv.gapThreshold)
	if err != nil {
		return err
	}

	_, err = rv.buildRep(repGame, rv.color, 1.0, rv.gapSkip, 0)

	return err
}
