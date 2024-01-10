/* Utility for creating pgn files */

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type IncrementPosture int

const (
	Increment IncrementPosture = iota + 1
	Delay
	None
)

type MkOpts struct {
	evalDepth int
	outfile   string
}

type MkCtx struct {
	opts         MkOpts
	tags         map[string]string
	openingGame  *chesstools.OpeningGame
	opening      string
	eco          string
	moveClocks   []string
	priorClock   []string
	in           *bufio.Reader
	resultReason string
}

func (pos IncrementPosture) String() string {
	switch pos {
	case None:
		return "No increment"
	case Increment:
		return "Increment (Fischer)"
	case Delay:
		return "Delay (Bronstein)"
	}

	return "<unknown>"
}

func NewMkCtx(optsIn MkOpts) *MkCtx {
	rv := &MkCtx{
		opts:       optsIn,
		tags:       make(map[string]string),
		moveClocks: make([]string, 0),
		priorClock: make([]string, 2),
		in:         bufio.NewReader(os.Stdin),
	}

	rv.openingGame = chesstools.NewOpeningGame()

	return rv
}

func main() {
	var opts MkOpts
	err := parseArgs(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgnmk: Failed to parse arguments: %v\n", err)
		os.Exit(1)
		return
	}

	mkCtx := NewMkCtx(opts)
	err = mkCtx.getPgnInputsInteractive()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgnmk: Failed to create PGN: %v\n", err)
		os.Exit(1)
		return
	}

	fmt.Printf("OUTPUT:\n%v", mkCtx.String())
}

func parseArgs(opts *MkOpts) error {
	f := flag.NewFlagSet("pgnmk", flag.ContinueOnError)

	f.IntVar(&opts.evalDepth, "evaldepth", 0, "<evalDepthInPlies>")
	err := f.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if len(f.Args()) > 1 {
		return fmt.Errorf("please specify only 1 PGN output file")
	} else if len(f.Args()) == 1 {
		opts.outfile = f.Args()[0]
	}

	return nil
}

func (mkCtx *MkCtx) getOneTag(key string, defaultVal string, required bool) {
	val := ""
	var err error
	for val == "" {
		if defaultVal != "" {
			fmt.Printf("%v [%v]: ", key, defaultVal)
		} else {
			fmt.Printf("%v: ", key)
		}
		val, err = mkCtx.in.ReadString('\n')
		if err != nil {
			panic(err)
			val = ""
			continue
		}
		val = strings.TrimSpace(val)
		if val == "" {
			val = defaultVal
		}
		if !required {
			break
		}
	}

	if val != "" {
		mkCtx.tags[key] = val
	}
}

func (mkCtx *MkCtx) String() string {
	tagKeys := []string{"Event", "Site", "Round", "Date", "Time", "White",
		"Black", "Result", "UTCDate", "UTCTime", "WhiteElo", "BlackElo",
		"Variant", "TimeControl", "ECO", "Opening", "Termination", "Annotator"}

	var sb strings.Builder
	for _, key := range tagKeys {
		val, ok := mkCtx.tags[key]
		if !ok || val == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%v \"%v\"]\n", key, val))
	}

	sb.WriteString("\n")

	g := mkCtx.openingGame.G
	mvs := g.Moves()
	pos := g.Positions()
	mvNum := 1
	notation := chess.AlgebraicNotation{}

	for idx := 0; idx < len(mvs); idx++ {
		var mvNumStr string

		if idx%2 == 0 {
			mvNumStr = fmt.Sprintf("%v", mvNum)
		} else {
			mvNumStr = fmt.Sprintf("%v..", mvNum)
		}

		mvDisp := notation.Encode(pos[idx], mvs[idx])

		sb.WriteString(fmt.Sprintf("%v. %v { [%%clk %v] } ", mvNumStr, mvDisp,
			mkCtx.moveClocks[idx]))

		if idx%2 != 0 {
			mvNum++
		}
	}

	if mkCtx.resultReason != "" {
		sb.WriteString(fmt.Sprintf("{ %v } ", mkCtx.resultReason))
	}

	sb.WriteString(g.Outcome().String())
	sb.WriteString("\n\n\n")

	return sb.String()
}

func (mkCtx *MkCtx) getPgnInputsInteractive() error {
	mkCtx.getOneTag("Event", "", true)
	mkCtx.getOneTag("Site", "", true)

	currentTime := time.Now()
	curDate := fmt.Sprintf("%v.%02v.%02v", currentTime.Year(),
		int(currentTime.Month()), currentTime.Day())
	defaultTime := "12:00:00"

	err := fmt.Errorf("once")
	for err != nil {
		mkCtx.getOneTag("Date", curDate, true)
		mkCtx.getOneTag("Time", defaultTime, false)
		err = mkCtx.populateUTCDateAndTime()
	}

	mkCtx.getOneTag("Round", "1", true)
	mkCtx.getOneTag("White", "", true)
	mkCtx.getOneTag("WhiteElo", "", false)
	mkCtx.getOneTag("Black", "", true)
	mkCtx.getOneTag("BlackElo", "", false)

	mkCtx.getTimeControl()

	mkCtx.tags["Variant"] = "Standard"
	mkCtx.tags["Annotator"] = "https://github.com/mikeb26/chesstools"

	mkCtx.getMovesAndClock(0)
	mkCtx.getOpeningAndEco()

	return nil
}

func (mkCtx *MkCtx) populateUTCDateAndTime() error {
	date, ok := mkCtx.tags["Date"]
	if !ok {
		return fmt.Errorf("Date is required")
	}
	dateParts := strings.Split(date, ".")
	if len(dateParts) != 3 {
		return fmt.Errorf("Unabled to parse date string '%v'", date)
	}
	var err error
	var year uint64
	year, err = strconv.ParseUint(dateParts[0], 10, 64)
	if err != nil {
		return fmt.Errorf("Unabled to parse year '%v': %w", dateParts[0], err)
	}
	var month uint64
	month, err = strconv.ParseUint(dateParts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("Unabled to parse month '%v': %w", dateParts[1], err)
	}
	var day uint64
	day, err = strconv.ParseUint(dateParts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("Unabled to parse day '%v': %w", dateParts[2], err)
	}

	var hour uint64
	var min uint64
	var second uint64
	timeStr, ok := mkCtx.tags["Time"]
	if !ok {
		hour = 0
		min = 0
		second = 0
	} else {
		timeParts := strings.Split(timeStr, ":")
		if len(timeParts) != 3 {
			return fmt.Errorf("Unabled to parse time string '%v'", timeStr)
		}
		hour, err = strconv.ParseUint(timeParts[0], 10, 64)
		if err != nil {
			return fmt.Errorf("Unabled to parse hour '%v': %w", timeParts[0], err)
		}
		min, err = strconv.ParseUint(timeParts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("Unabled to parse minute '%v': %w", timeParts[1], err)
		}
		second, err = strconv.ParseUint(timeParts[2], 10, 64)
		if err != nil {
			return fmt.Errorf("Unabled to parse second '%v': %w", timeParts[2], err)
		}
	}

	timestamp := time.Date(int(year), time.Month(month), int(day), int(hour),
		int(min), int(second), 0, time.Local)
	mkCtx.tags["UTCDate"] = fmt.Sprintf("%v.%02v.%02v",
		timestamp.UTC().Year(), int(timestamp.UTC().Month()),
		timestamp.UTC().Day())
	mkCtx.tags["UTCTime"] = fmt.Sprintf("%02v:%02v:%02v",
		timestamp.UTC().Hour(), timestamp.UTC().Minute(),
		timestamp.UTC().Second())

	timestamp.UTC().Date()

	return nil
}

func (mkCtx *MkCtx) getUserIntVal(val *int64, defaultVal int64,
	fmtStr string) error {

	*val = defaultVal
	if fmtStr != "" {
		fmt.Printf(fmtStr, defaultVal)
	}
	valStr, err := mkCtx.in.ReadString('\n')
	if err != nil {
		return err
	}
	valStr = strings.TrimSpace(valStr)
	if valStr != "" {
		*val, err = strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return err
		}
	}

	return nil
}

func (mkCtx *MkCtx) getUserStringVal(val *string, defaultVal string,
	fmtStr string) error {

	*val = defaultVal
	if fmtStr != "" {
		fmt.Printf(fmtStr, defaultVal)
	}
	valStr, err := mkCtx.in.ReadString('\n')
	if err != nil {
		return err
	}
	valStr = strings.TrimSpace(valStr)
	if valStr != "" {
		*val = valStr
	}

	return nil
}

func (mkCtx *MkCtx) getTimeControl() error {
	var numPeriods int64
	err := mkCtx.getUserIntVal(&numPeriods, 1, "How many time periods? [%v]: ")
	if err != nil {
		return err
	}

	var timeControlSb strings.Builder

	var firstPeriodTimeInSec int64

	for idx := 0; idx < int(numPeriods); idx++ {
		var movesInPeriod int64

		if idx != int(numPeriods)-1 {
			err := mkCtx.getUserIntVal(&movesInPeriod, 40,
				"How many moves in period %v? [%v]: ")
			if err != nil {
				return err
			}
		}

		var periodTimeInSec int64
		err := mkCtx.getUserIntVal(&periodTimeInSec, 5400, /* 1hr 30min */
			"How long is period in seconds? [%v]: ")
		if err != nil {
			return err
		}

		if idx == 0 {
			firstPeriodTimeInSec = periodTimeInSec
		}
		pos, incrInSec, err := mkCtx.getIncrementPostureAndVal(idx + 1)
		if err != nil {
			return err
		}

		if idx != int(numPeriods)-1 {
			timeControlSb.WriteString(fmt.Sprintf("%v/", movesInPeriod))
		}
		timeControlSb.WriteString(fmt.Sprintf("%v", periodTimeInSec))
		if pos == Delay {
			timeControlSb.WriteString(fmt.Sprintf("d%v", incrInSec))
		} else {
			timeControlSb.WriteString(fmt.Sprintf("+%v", incrInSec))
		}
		if idx != int(numPeriods)-1 {
			timeControlSb.WriteString(":")
		}
	}

	mkCtx.tags["TimeControl"] = timeControlSb.String()

	mkCtx.initPriorClocks(int(firstPeriodTimeInSec))

	return nil
}

func (mkCtx *MkCtx) getIncrementPostureAndVal(period int) (IncrementPosture,
	int, error) {

	fmt.Printf("\n  1. Increment (Fischer)\n")
	fmt.Printf("  2. Delay (Bronstein)\n")
	fmt.Printf("  3. No increment\n")

	var err error
	var posture int64
	for posture == 0 {
		err = mkCtx.getUserIntVal(&posture, 1,
			"Enter the period increment type (pick number from above) [%v]: ")
		if err != nil {
			return None, 0, err
		}
		if posture < 1 || posture > 3 {
			posture = 0
		}
	}

	var val int64
	pos := IncrementPosture(posture)
	if pos != None {
		err = mkCtx.getUserIntVal(&val, 5,
			"Enter increment time in seconds [%v]: ")
		if err != nil {
			return None, 0, err
		}
	}

	return pos, int(val), nil
}

func (mkCtx *MkCtx) getMovesAndClock(halfMoveCount int) error {
	openingGame := mkCtx.openingGame
	if openingGame.G.Outcome() != chess.NoOutcome {
		mkCtx.assignNormalResult()
		return nil
	}

	var clockVal string

	err := fmt.Errorf("once")
	for err != nil {
		fmt.Printf("\n  Opening: %v (%v)\n", openingGame.String(), openingGame.Eco)
		fmt.Printf("  PGN:%v\n  FEN: \"%v\"\n", openingGame.G.String(),
			openingGame.G.Position().XFENString())
		fmt.Printf("%v", openingGame.G.Position().Board().Draw())

		fmt.Printf("Enter %v Move %v (or timeout/resign/end): ",
			openingGame.Turn().Name(), halfMoveCount/2+1)

		var mv string
		err = mkCtx.getUserStringVal(&mv, "", "")
		if err != nil {
			return err
		}

		if mv == "end" {
			return mkCtx.assignResultInteractive()
		} else if mv == "resign" {
			mkCtx.assignResignResult()
			return nil
		} else if mv == "timeout" {
			mkCtx.assignTimeoutResult()
			return nil
		}

		if mv != "" {
			mkCtx.openingGame = chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv)
		} else {
			err = fmt.Errorf("empty move")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "\n*pgnmk: invalid move '%v' in position*\n", mv)
		}
	}

	clockVal = mkCtx.priorClock[openingGame.Turn()-1]
	fmt.Printf("\nEnter clock after %v Move %v [%v]: ",
		openingGame.Turn().Name(), halfMoveCount/2+1, clockVal)
	err = mkCtx.getUserStringVal(&clockVal, clockVal, "")
	if err != nil {
		return err
	}
	mkCtx.priorClock[openingGame.Turn()-1] = clockVal
	mkCtx.moveClocks = append(mkCtx.moveClocks, clockVal)

	return mkCtx.getMovesAndClock(halfMoveCount + 1)
}

func (mkCtx *MkCtx) initPriorClocks(firstPeriodTimeInSec int) {
	hours := firstPeriodTimeInSec / 3600
	firstPeriodTimeInSec -= (hours * 3600)

	mins := firstPeriodTimeInSec / 60
	secs := firstPeriodTimeInSec % 60

	mkCtx.priorClock[0] = fmt.Sprintf("%v:%02v:%02v", hours, mins, secs)
	mkCtx.priorClock[1] = mkCtx.priorClock[0]
}

func (mkCtx *MkCtx) assignNormalResult() {
	g := mkCtx.openingGame.G

	mkCtx.tags["Result"] = g.Outcome().String()
	mkCtx.tags["Termination"] = "Normal"

	mkCtx.resultReason = fmt.Sprintf("%v wins by checkmate.",
		g.Position().Turn().Other().Name())
}

func (mkCtx *MkCtx) assignResignResult() {
	g := mkCtx.openingGame.G

	g.Resign(g.Position().Turn())
	mkCtx.tags["Result"] = g.Outcome().String()
	mkCtx.tags["Termination"] = "Normal"

	mkCtx.resultReason = fmt.Sprintf("%v resigns.", g.Position().Turn().Name())
}

func (mkCtx *MkCtx) assignTimeoutResult() {
	g := mkCtx.openingGame.G

	g.Resign(g.Position().Turn())
	mkCtx.tags["Result"] = g.Outcome().String()
	mkCtx.tags["Termination"] = "Time forfeit"

	mkCtx.resultReason = fmt.Sprintf("%v wins on time.",
		g.Position().Turn().Other().Name())
}

func (mkCtx *MkCtx) assignResultInteractive() error {
	fmt.Printf("\n  1. White won\n")
	fmt.Printf("  2. Black won\n")
	fmt.Printf("  3. Draw\n")
	fmt.Printf("  4. Game is unfinished\n")

	var val int64
	for val == 0 {
		err := mkCtx.getUserIntVal(&val, 4,
			"Result? (pick number from above): [%v]")
		if err != nil {
			return err
		}
		if val < 1 || val > 4 {
			val = 0
		}
	}

	switch val {
	case 1:
		mkCtx.tags["Result"] = chess.WhiteWon.String()
	case 2:
		mkCtx.tags["Result"] = chess.BlackWon.String()
	case 3:
		mkCtx.tags["Result"] = chess.Draw.String()
		mkCtx.resultReason = "Draw by mutual agreement."
	case 4:
		mkCtx.tags["Result"] = chess.NoOutcome.String()
	}

	return nil
}

func (mkCtx *MkCtx) getOpeningAndEco() {
	mkCtx.tags["ECO"] = mkCtx.openingGame.Eco
	mkCtx.tags["Opening"] = mkCtx.openingGame.String()
}
