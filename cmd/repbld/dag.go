package main

import (
	"fmt"
	"io"
	"time"

	"github.com/mikeb26/chesstools"
	"github.com/notnil/chess"
)

type OutputMode int

const (
	Flattened OutputMode = iota
	Consolidated
)

type DagNode struct {
	position *chess.Position
	// indexed by move
	children                map[string]*DagNode
	numParents              int
	moveNum                 int
	alreadyEmitted          bool
	childrenAlreadyComputed bool
	openingName             string
	openingNameHasSuffix    bool
	moveListSet             MoveListAndStartFENSet
	nodeId                  int
}

type Dag struct {
	root     *DagNode
	numNodes int // unique positions

	// indexed by FEN
	nodeMap    map[string]*DagNode
	outputMode OutputMode
	repColor   chess.Color
}

func NewDag(repColorIn chess.Color, outputModeIn OutputMode) *Dag {
	dag := &Dag{
		root: &DagNode{
			position:                chess.StartingPosition(),
			children:                make(map[string]*DagNode),
			numParents:              0,
			moveNum:                 1,
			alreadyEmitted:          false,
			childrenAlreadyComputed: false,
			openingName:             "",
			openingNameHasSuffix:    false,
			moveListSet:             NewMoveListAndStartFENSet(),
			nodeId:                  0,
		},
		numNodes:   1,
		nodeMap:    make(map[string]*DagNode),
		outputMode: outputModeIn,
		repColor:   repColorIn,
	}

	fen := dag.root.position.XFENString()
	dag.nodeMap[fen] = dag.root

	return dag
}

func (dag *Dag) upsertNode(parent *DagNode, pos *chess.Position,
	mv string) *DagNode {

	fen := pos.XFENString()

	// have to check if the dag has the node first
	// if the dag does have the node, then have to check if this parent
	// has the node
	dagNode, ok := dag.nodeMap[fen]
	if !ok {
		// node does not yet exist anywhere in the Dag
		dagNode = &DagNode{
			position:                pos,
			children:                make(map[string]*DagNode),
			numParents:              1,
			moveNum:                 parent.moveNum,
			alreadyEmitted:          false,
			childrenAlreadyComputed: false,
			openingName:             "",
			openingNameHasSuffix:    false,
			moveListSet:             NewMoveListAndStartFENSet(),
			nodeId:                  dag.numNodes,
		}
		dagNode.openingName, dagNode.openingNameHasSuffix =
			dag.getOpeningName(parent, dagNode, mv)
		if pos.Turn() == chess.White {
			dagNode.moveNum++
		}

		if mv == "" {
			panic("BUG: empty mv during node insertion")
		}
		parent.children[mv] = dagNode
		dag.nodeMap[fen] = dagNode
		dag.numNodes++
		return dagNode
	}

	if parent != nil {
		if mv == "" {
			panic("BUG: empty mv during node insertion")
		}
		// the Dag already has this node, but it may not yet have this
		// parent
		dagNode2, ok := parent.children[mv]
		if ok {
			// node already exists in both the Dag and in its parent's children
			// map, so no-op
			if dagNode != dagNode2 {
				panic("BUG: distinct dag nodes for same position")
			}
			return dagNode
		}

		// the Dag has this node, but this parent doesnt
		dagNode.numParents++
		parent.children[mv] = dagNode
	}

	return dagNode
}

func (dag *Dag) addNodesFromGame(game *chess.Game) {
	moves := game.Moves()
	positions := game.Positions()
	if len(moves) > len(positions) {
		panic(fmt.Sprintf("can't parse game %v", game))
	}

	var parentNode *DagNode
	var mv string

	for idx, pos := range game.Positions() {
		if idx == 0 {
			parentNode = nil
			mv = ""
		} else {
			encoder := chess.AlgebraicNotation{}
			mv = encoder.Encode(parentNode.position, moves[idx-1])
		}
		dagNode := dag.upsertNode(parentNode, pos, mv)
		parentNode = dagNode
	}
}

func (dag *Dag) emit(output io.Writer) {
	moveList := NewMoveListAndStartFEN()

	dag.computeMoveLists(dag.root, moveList)

	dag.emitNode(output, dag.root)
}

func (dag *Dag) computeMoveLists(node *DagNode, moveList MoveListAndStartFEN) {
	if len(node.children) == 0 {
		node.moveListSet.moveLists = append(node.moveListSet.moveLists, moveList)
		return
	}

	fen := node.position.XFENString()

	numChildren := len(node.children)
	if dag.outputMode == Consolidated && node != dag.root {
		if numChildren > 1 || node.numParents > 1 || node.childrenAlreadyComputed {
			node.moveListSet.moveLists = append(node.moveListSet.moveLists, moveList)
			moveList = NewMoveListAndStartFEN()
			moveList.fen = fen
			moveList.turn = node.position.Turn()
			moveList.moveNum = node.moveNum
		}

		// in Flattened mode we should emit children again since moveList will
		// differ with a distinct parent lineage and we want the fully expanded
		// combinatorics. in Consolidated mode that lineage will get squelched via
		// FEN
		if node.childrenAlreadyComputed {
			return
		}
	}

	for mv, childNode := range node.children {
		childMoveList := moveList.clone()
		childMoveList.moves = append(childMoveList.moves, mv)

		dag.computeMoveLists(childNode, childMoveList)
	}
	node.childrenAlreadyComputed = true
}

func (dag *Dag) emitNode(output io.Writer, node *DagNode) {
	numChildren := len(node.children)

	if node.alreadyEmitted {
		return
	}
	if numChildren == 0 {
		dag.emitGameToOutput(output, node)
		return
	}
	if dag.outputMode == Consolidated && node != dag.root {
		if numChildren > 1 || node.numParents > 1 {
			dag.emitGameToOutput(output, node)
		}
	}

	for _, childNode := range node.children {
		dag.emitNode(output, childNode)
	}
}

func (dag *Dag) emitGameHeadersToOutput(output io.Writer, node *DagNode,
	fen string) {

	fmt.Fprintf(output, "[Event \"%v\"]\n", node.openingName)
	fmt.Fprintf(output, "[Result \"%v\"]\n", "*")

	currentTime := time.Now()
	fmt.Fprintf(output, "[UTCDate \"%v\"]\n", fmt.Sprintf("%v.%02v.%02v",
		currentTime.UTC().Year(), int(currentTime.UTC().Month()),
		currentTime.UTC().Day()))
	fmt.Fprintf(output, "[UTCTime \"%v\"]\n", fmt.Sprintf("%02v:%02v:%02v",
		currentTime.UTC().Hour(), currentTime.UTC().Minute(),
		currentTime.UTC().Second()))
	fmt.Fprintf(output, "[Variant \"%v\"]\n", "Standard")
	fmt.Fprintf(output, "[Annotator \"%v\"]\n",
		"https://github.com/mikeb26/chesstools")
	if fen != "" {
		fmt.Fprintf(output, "[FEN \"%v\"]\n", fen)
	}
}

func (dag *Dag) emitGameToOutput(output io.Writer, node *DagNode) error {
	if dag.outputMode == Consolidated &&
		node.moveListSet.allMoveListsHaveSameFEN() {
		dag.emitGameHeadersToOutput(output, node,
			node.moveListSet.moveLists[0].fen)
		fmt.Fprintf(output, "\n%v *\n\n\n", node.moveListSet.String())
	} else {
		for _, moveList := range node.moveListSet.moveLists {
			dag.emitGameHeadersToOutput(output, node, moveList.fen)
			fmt.Fprintf(output, "\n%v *\n\n\n", moveList.String())
		}
	}

	node.alreadyEmitted = true

	return nil
}

func (dag *Dag) getOpeningName(parent *DagNode, node *DagNode,
	mv string) (string, bool) {

	opening := chesstools.GetOpeningName(node.position.XFENString())
	if opening != "" {
		return opening, false
	}

	// only allow a single move suffix
	if parent.openingNameHasSuffix {
		return parent.openingName, true
	}

	if dag.repColor == parent.position.Turn() {
		return parent.openingName, false
	}

	mvNumStr := fmt.Sprintf("%v.", parent.moveNum)
	if parent.position.Turn() == chess.Black {
		mvNumStr = fmt.Sprintf("%v..", mvNumStr)
	}
	opening = fmt.Sprintf("%v, %v %v", parent.openingName, mvNumStr, mv)

	return opening, true
}
