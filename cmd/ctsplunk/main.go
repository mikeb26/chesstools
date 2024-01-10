/* Utility for investigating which players have had a list of positions
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type SplunkOpts struct {
	fenColorList string
	playerList   []string
	opponent     string
}

const (
	MaxGames = 100
	/* https://lichess.org/api#tag/Opening-Explorer/operation/openingExplorerMaster
	 * states recentGames is limited to 4
	 */
	MinGames = 4
	BaseUrl  = "https://lichess.org"
)

var ErrTooManyGames = errors.New("too many games from this position")
var ErrNoGames = errors.New("no games from this position")

func encodeKey(fen string, color string) string {
	return fmt.Sprintf("%s:%s", fen, color)
}

func decodeKey(key string) (string, chess.Color) {
	parts := strings.Split(key, ":")

	var c chess.Color
	if strings.ToLower(parts[1]) == "white" {
		c = chess.White
	} else {
		c = chess.Black
	}

	return parts[0], c
}

func parseArgs(opts *SplunkOpts) error {
	f := flag.NewFlagSet("ctsplunk", flag.ContinueOnError)

	var playerList string
	f.StringVar(&opts.fenColorList, "fencolorlist", "", "<fen1:color1>[,<fen2:color2>...]")
	f.StringVar(&playerList, "playerlist", "", "<player1>[,<player2>...]")
	f.StringVar(&opts.opponent, "opponent", "", "<player1>")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if opts.fenColorList == "" && playerList == "" {
		return fmt.Errorf("must specify at least 1 of --fencolorlist or --playerlist")
	}
	if playerList != "" {
		opts.playerList = strings.Split(playerList, ",")
	} else {
		opts.playerList = make([]string, 0)
	}

	return err
}

func main() {
	var opts SplunkOpts
	err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctsplunk: %v\n", err)
		os.Exit(1)
		return
	}

	_, err = mainWork(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctsplunk: %v\n", err)
		os.Exit(1)
		return
	}
}

func mainWork(opts *SplunkOpts) ([]string, error) {
	playerList := make([]string, 0)

	fenColor2InfosMap := make(map[string][]chesstools.GameInfo)

	// 2-pass algorithm. in the first pass, lookup each position from the
	// lichess database and compute an initial (possibly incomplete) list of
	// gameInfos to work from. after computing the intersection of players from
	// those, in the 2nd pass revisit the positions for which gameInfos which
	// were not retrieved in the first pass. for each of these, use the
	// player games endpoint to determine whether a player has had the position.
	// Incompleteness in the first pass occurs when the number of games in the
	// lichess database from a given position exceeds MaxGames. This is done
	// in order to avoid excessive use of the lichess api. e.g. we should fail
	// and return an error if a user asks for a list of players who have had
	// the initial position and the position after 1. e4.

	// 1st pass
	if opts.fenColorList != "" {
		for _, fenAndColor := range strings.Split(opts.fenColorList, ",") {
			fenAndColorParts := strings.Split(fenAndColor, ":")
			if len(fenAndColorParts) != 2 {
				return playerList, fmt.Errorf("Could not parse position&color %v within %v",
					fenAndColor, opts.fenColorList)
			}
			fen := fenAndColorParts[0]
			color := fenAndColorParts[1]
			key := encodeKey(fen, color)
			gameInfos, err := getFENGameInfos(fen)
			if errors.Is(err, ErrTooManyGames) {
				fenColor2InfosMap[key] = nil // nil is a sentinel used in the 2nd pass
			} else if err != nil {
				return playerList, err
			} // else no err

			fenColor2InfosMap[key] = gameInfos
		}
	}

	playerList = computePlayerList(opts.playerList, fenColor2InfosMap)

	// 2nd pass
	for key, gameInfos := range fenColor2InfosMap {
		fen, color := decodeKey(key)
		if gameInfos == nil {
			gameInfos, err := getFENGameInfosByPlayers(fen, color, playerList)
			if err != nil {
				return playerList, err
			}
			fenColor2InfosMap[key] = gameInfos
		}
	}

	playerList = computePlayerList(opts.playerList, fenColor2InfosMap)

	// check against opponent if specified
	var err error
	playerList, err = trimByOpponent(playerList, opts.opponent)
	if err != nil {
		return playerList, err
	}

	// output
	fmt.Printf("The following players meet all search criteria:\n  %v\n",
		playerList)

	gameCount := 0
	for key, gameInfos := range fenColor2InfosMap {
		fen, color := decodeKey(key)

		fmt.Printf("\nSample of games where one of these players has had FEN '%v' as %v:\n", fen, color)

		for _, gi := range gameInfos {
			if (color == chess.Black && !contains(playerList, gi.Black.Name)) ||
				(color == chess.White && !contains(playerList, gi.White.Name)) {
				continue
			}
			gameCount++
			fmt.Printf("  Game%v:\n", gameCount)
			fmt.Printf("    Date: %v\n", gi.Month)
			fmt.Printf("    Url: %v\n", fmt.Sprintf("%v/%v", BaseUrl, gi.Id))
			fmt.Printf("    Mode: %v\n", gi.Mode)
			fmt.Printf("    Speed: %v\n", gi.Speed)
			fmt.Printf("    Winner: %v\n", gi.Winner)
			fmt.Printf("    Black: %v (%v)\n", gi.Black.Name, gi.Black.Rating)
			fmt.Printf("    White: %v (%v)\n", gi.White.Name, gi.White.Rating)
		}
	}

	return playerList, nil
}

func contains(strList []string, str string) bool {
	for _, cur := range strList {
		if cur == str {
			return true
		}
	}

	return false
}

func getFENGameInfos(fen string) ([]chesstools.GameInfo, error) {
	openingGame := chesstools.NewOpeningGame().WithFEN(fen).WithAllSpeeds(true).WithFullRatingRange(true).WithTopReplies(true)

	return getGameInfos(openingGame, false)
}

func getGameInfos(openingGame *chesstools.OpeningGame,
	sampleOk bool) ([]chesstools.GameInfo, error) {

	total := openingGame.OpeningResp.Total()
	if !sampleOk && total > MaxGames {
		// guards against excessive use of lichess api
		return nil, ErrTooManyGames
	} else if total == 0 {
		return nil, ErrNoGames
	} else if sampleOk || total <= MinGames {
		return openingGame.OpeningResp.TopGames, nil
	}

	// else MinGames < total <= MaxGames
	gameInfos := make([]chesstools.GameInfo, 0)
	for _, mv := range openingGame.OpeningResp.Moves {
		childGame :=
			chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv.San).WithAllSpeeds(true).WithTopReplies(true).WithEval(true)
		childGameInfos, err := getGameInfos(childGame, sampleOk)
		if errors.Is(err, ErrNoGames) {
			continue
		} else if err != nil {
			return nil, err
		}
		gameInfos = append(gameInfos, childGameInfos...)
	}

	return gameInfos, nil
}

func computePlayerList(initPlayerList []string,
	fenColor2InfosMap map[string][]chesstools.GameInfo) []string {

	playerPosCount := make(map[string]int)
	totalPositions := 0

	for key, gameInfos := range fenColor2InfosMap {
		_, color := decodeKey(key)
		if gameInfos == nil {
			continue
		}

		totalPositions++

		playerSeenInThisPos := make(map[string]bool)
		for _, gi := range gameInfos {
			var player string
			if color == chess.White {
				player = gi.White.Name
			} else {
				player = gi.Black.Name
			}

			_, ok := playerSeenInThisPos[player]
			if ok {
				// player has had this position in multiple games
				continue
			}

			curCount, ok := playerPosCount[player]
			if !ok {
				curCount = 0
			}
			playerPosCount[player] = curCount + 1
			playerSeenInThisPos[player] = true
		}
	}

	playerList := make([]string, 0)
	if len(fenColor2InfosMap) == 0 || totalPositions == 0 {
		playerList = initPlayerList
	} else {
		for player, posCount := range playerPosCount {
			if posCount != totalPositions {
				continue
			}
			if len(initPlayerList) != 0 && !contains(initPlayerList, player) {
				continue
			}

			playerList = append(playerList, player)
		}
	}

	return playerList
}

func getFENGameInfosByPlayers(fen string,
	color chess.Color, playerList []string) ([]chesstools.GameInfo, error) {

	gameInfos := make([]chesstools.GameInfo, 0)

	numPlayers := len(playerList)

	for idx, player := range playerList {
		fmt.Fprintf(os.Stderr, "\nChecking whether player %v has had FEN '%v' as %v (%v of %v)...",
			player, fen, color, idx+1, numPlayers)
		openingGame := chesstools.NewOpeningGame().WithFEN(fen).WithOpponent(player, color).WithAllSpeeds(true).WithFullRatingRange(true).WithTopReplies(true)

		playerGameInfos, err := getGameInfos(openingGame, true)
		if errors.Is(err, ErrNoGames) {
			continue
		} else if err != nil {
			return nil, err
		}

		gameInfos = append(gameInfos, playerGameInfos...)
	}

	if len(gameInfos) == 0 {
		return nil, ErrNoGames
	}

	return gameInfos, nil
}

func trimByOpponent(playerList []string, opponent string) ([]string, error) {

	if opponent == "" {
		return playerList, nil
	}

	outList := make([]string, 0)

	for _, player := range playerList {
		numGames, err := chesstools.GetCrossTable(player, opponent)
		if err != nil {
			return outList, err
		}
		if numGames == 0 {
			continue
		}
		outList = append(outList, player)
	}

	return outList, nil
}
