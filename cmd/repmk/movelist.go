package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/corentings/chess/v2"
	"github.com/mikeb26/chesstools"
)

type MoveListAndStartFEN struct {
	moves   []string
	fen     string
	turn    chess.Color
	moveNum int
}

type MoveListAndStartFENSet struct {
	moveLists []MoveListAndStartFEN
}

func (moveList MoveListAndStartFEN) String() string {
	var sb strings.Builder

	curMoveNum := moveList.moveNum
	curTurn := moveList.turn

	for idx, mv := range moveList.moves {
		if idx == 0 && curTurn == chess.Black {
			sb.WriteString(fmt.Sprintf("%v...", curMoveNum))
		} else if curTurn == chess.White {
			if idx != 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(fmt.Sprintf("%v.", curMoveNum))
		}
		sb.WriteString(fmt.Sprintf(" %v", mv))
		curTurn = curTurn.Other()
		if curTurn == chess.White {
			curMoveNum++
		}
	}

	return sb.String()
}

func NewMoveListAndStartFEN() MoveListAndStartFEN {
	return MoveListAndStartFEN{
		moves:   make([]string, 0),
		fen:     "",
		turn:    chess.White,
		moveNum: 1,
	}
}

func (moveList MoveListAndStartFEN) clone() MoveListAndStartFEN {
	ret := NewMoveListAndStartFEN()

	ret.fen = moveList.fen
	ret.turn = moveList.turn
	ret.moveNum = moveList.moveNum
	ret.moves = make([]string, len(moveList.moves))
	copy(ret.moves, moveList.moves)

	return ret
}

func NewMoveListAndStartFENSet() MoveListAndStartFENSet {
	return MoveListAndStartFENSet{
		moveLists: make([]MoveListAndStartFEN, 0),
	}
}

func (moveListSet MoveListAndStartFENSet) String() string {
	if len(moveListSet.moveLists) == 1 {
		return moveListSet.moveLists[0].String()
	}

	firstMvList := moveListSet.moveLists[0]
	fen := firstMvList.fen
	fen, err := chesstools.NormalizeFEN(fen)
	if err != nil {
		panic("Can't parse fen")
	}
	turn := firstMvList.turn
	moveNum := firstMvList.moveNum
	numMoves := len(firstMvList.moves)

	var sb strings.Builder

	// sanity checks
	for _, mvList := range moveListSet.moveLists {
		curFen, err := chesstools.NormalizeFEN(mvList.fen)
		if err != nil {
			panic("Can't parse fen")
		}
		if curFen != fen {
			for idx, mvList2 := range moveListSet.moveLists {
				fmt.Fprintf(os.Stderr, "mvList[%v] fen:%v moves:%v\n", idx,
					mvList2.fen, mvList2)
			}
			panic(fmt.Sprintf("moveList fen does not match\n\t1st:%v\n\tcur:%v",
				fen, curFen))
		}
		if mvList.turn != turn {
			panic(fmt.Sprintf("moveList turn %v does not match %v", mvList.turn,
				turn))
		}
		if mvList.moveNum != moveNum {
			panic(fmt.Sprintf("moveList moveNum %v does not match %v",
				mvList.moveNum, moveNum))
		}
		if len(mvList.moves) != numMoves {
			panic(fmt.Sprintf("moveList numMoves %v does not match %v",
				len(mvList.moves), numMoves))
		}
	}

	if turn == chess.Black {
		sb.WriteString(fmt.Sprintf("%v... %v", moveNum, firstMvList.moves[0]))
	} else {
		sb.WriteString(fmt.Sprintf("%v. %v", moveNum, firstMvList.moves[0]))
	}

	for idx, mvList := range moveListSet.moveLists {
		if idx == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf(" (%v)", mvList.String()))
	}

	cdrFirstMvList := NewMoveListAndStartFEN()
	cdrFirstMvList.turn = firstMvList.turn.Other()
	cdrFirstMvList.moveNum = firstMvList.moveNum
	if cdrFirstMvList.turn == chess.White {
		cdrFirstMvList.moveNum++
	}
	cdrFirstMvList.moves = make([]string, numMoves-1)
	copy(cdrFirstMvList.moves, firstMvList.moves[1:numMoves])

	sb.WriteString(fmt.Sprintf(" %v", cdrFirstMvList.String()))

	return sb.String()
}

func (moveListSet MoveListAndStartFENSet) allMoveListsHaveSameFEN() bool {
	fen := ""
	curFen := ""
	var err error

	for _, mvList := range moveListSet.moveLists {
		if fen == "" {
			fen, err = chesstools.NormalizeFEN(mvList.fen)
			curFen = fen
		} else {
			curFen, err = chesstools.NormalizeFEN(mvList.fen)
		}
		if err != nil {
			return false
		}
		if curFen != fen {
			return false
		}
	}

	return true
}
