package main

import (
	"fmt"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

func (rv *RepValidator) selectMove(openingGame *chesstools.OpeningGame,
	totalPct float64) (string, error) {

	normalizedFen, err := chesstools.NormalizeFEN(openingGame.G.FEN())
	if err != nil {
		return "", fmt.Errorf("Failed to normalize FEN %v: %w",
			openingGame.G.FEN(), err)
	}

	moveMapVal, ok := rv.moveMap[normalizedFen]
	if !ok {
		return "", nil
	}

	return moveMapVal.move, nil
}

func (rv *RepValidator) buildRep(openingGame *chesstools.OpeningGame,
	color chess.Color, totalPct float64, gapSkip int,
	stackDepth int) (bool, error) {

	if openingGame.Turn() == color {
		mv, err := rv.selectMove(openingGame, totalPct)
		if err != nil {
			return false, err
		}
		if mv == "" {
			if gapSkip == 0 && totalPct < 0.999 {
				fmt.Printf("  gap:%v(%v) pct:%v\n", openingGame.G.String(),
					openingGame.String(), chesstools.PctS2(totalPct))
			}
			return false, nil
		}
		childGame := chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv).WithThreshold(openingGame.Threshold).WithTopReplies(true)
		return rv.buildRep(childGame, color, totalPct, gapSkip, stackDepth+1)
	} // else

	pushedOne := false
	total := openingGame.OpeningResp.Total()
	for _, mv := range openingGame.OpeningResp.Moves {
		mvTotal := mv.Total()
		if chesstools.Pct(mvTotal, total)*totalPct < rv.opts.gapThreshold {
			continue
		}
		pushedOne = true

		childGame := chesstools.NewOpeningGame().WithParent(openingGame).WithMove(mv.San).WithThreshold(openingGame.Threshold).WithTopReplies(true)
		var childTotalPct float64
		var childGapSkip int
		childGapSkip = gapSkip
		if childGapSkip > 0 {
			childGapSkip--
			childTotalPct = totalPct
		} else {
			childTotalPct = totalPct * chesstools.Pct(mvTotal, total)
		}
		_, err := rv.buildRep(childGame, color, childTotalPct, childGapSkip,
			stackDepth+1)
		if err != nil {
			return false, err
		}
	}

	return pushedOne, nil
}

func (rv *RepValidator) checkForGaps() error {
	openingGame := chesstools.NewOpeningGame().WithThreshold(rv.opts.gapThreshold).WithTopReplies(true)

	_, err := rv.buildRep(openingGame, rv.opts.color, 1.0, rv.opts.gapSkip, 0)

	return err
}
