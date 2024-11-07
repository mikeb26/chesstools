/* Copyright © 2021-2024 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this package for license terms
 */
package chesstools

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
)

const (
	KiB = 1024
	MiB = 1024 * KiB

	DefaultEvalTimeInSec = 300
	DefaultDepth         = -1 // infinite
	UnknownSearchTime    = 0.0
	UnknownEngVer        = 0.0
	FileNamePrefix       = "fen."
	CacheFileDir         = "cache"
)

var ErrCacheMiss = errors.New("cache miss")
var ErrCacheStale = errors.New("cache stale")

type EvalType uint64

const (
	EvalTypeLocalStockfish EvalType = iota
	EvalTypeLichess

	EvalTypeInvalid // must be last
)

func (t EvalType) String() string {
	var ret string

	switch t {
	case EvalTypeLocalStockfish:
		ret = "local stockfish"
	case EvalTypeLichess:
		ret = "lichess"
	case EvalTypeInvalid:
	default:
		ret = "invalid"
	}

	return ret
}

type EvalResult struct {
	CP                  int
	WinPct              float32
	DrawPct             float32
	LossPct             float32
	Mate                int
	BestMove            string
	Depth               int
	EngVersion          float64
	KNPS                string
	SearchTimeInSeconds float64
	Type                EvalType
}

type EvalCtx struct {
	turn          chess.Color
	moveNum       uint
	pgnFile       string
	fen           string
	numThreads    uint64 // default == num CPU hyperthreads
	hashSizeInMiB uint64 // default == 50% system RAM
	evalTimeInSec uint   // default == 5 minutes
	evalDepth     int    // default == infinite
	g             *chess.Game
	cacheOnly     bool
	staleOk       bool
	cloudCache    bool
	doLazyInit    bool

	engine     *uci.Engine
	engVersion float64
	position   *chess.Position
}

func (evalCtx *EvalCtx) Close() {
	if evalCtx.engine != nil {
		evalCtx.engine.Close()
		evalCtx.engine = nil
	}
}

func NewEvalCtx(cacheOnlyIn bool) *EvalCtx {
	rv := &EvalCtx{}

	rv.turn = chess.White
	rv.moveNum = 0
	rv.pgnFile = ""
	rv.fen = ""
	rv.numThreads = uint64(runtime.NumCPU())
	rv.hashSizeInMiB = (getSystemMem() * 2) / (MiB * 4)
	rv.evalTimeInSec = DefaultEvalTimeInSec
	rv.evalDepth = DefaultDepth
	rv.g = nil
	rv.position = nil
	rv.cacheOnly = cacheOnlyIn
	rv.staleOk = cacheOnlyIn
	rv.cloudCache = true
	rv.doLazyInit = false

	var err error
	rv.engine, err = uci.New("stockfish")
	if err != nil {
		panic("Unable to initialize stockfish")
	}

	return rv
}

func (evalCtx *EvalCtx) WithPgnFile(pgnFile string) *EvalCtx {
	evalCtx.pgnFile = pgnFile
	return evalCtx
}

func (evalCtx *EvalCtx) WithCacheOnly() *EvalCtx {
	evalCtx.cacheOnly = true
	evalCtx.staleOk = true
	return evalCtx
}

func (evalCtx *EvalCtx) WithStaleOk() *EvalCtx {
	evalCtx.staleOk = true
	return evalCtx
}

func (evalCtx *EvalCtx) WithoutCloudCache() *EvalCtx {
	evalCtx.cloudCache = false
	return evalCtx
}

func (evalCtx *EvalCtx) WithFEN(fen string) *EvalCtx {
	evalCtx.fen = fen
	return evalCtx
}

func (evalCtx *EvalCtx) WithTurn(turn chess.Color) *EvalCtx {
	evalCtx.turn = turn
	return evalCtx
}

func (evalCtx *EvalCtx) WithMoveNum(moveNum uint) *EvalCtx {
	evalCtx.moveNum = moveNum
	return evalCtx
}

func (evalCtx *EvalCtx) WithThreads(numThreads uint64) *EvalCtx {
	evalCtx.numThreads = numThreads
	return evalCtx
}

func (evalCtx *EvalCtx) WithHashSize(hashSizeInMiB uint64) *EvalCtx {
	evalCtx.hashSizeInMiB = hashSizeInMiB
	return evalCtx
}

func (evalCtx *EvalCtx) WithEvalTime(evalTimeInSec uint) *EvalCtx {
	evalCtx.evalTimeInSec = evalTimeInSec
	return evalCtx
}

func (evalCtx *EvalCtx) WithEvalDepth(evalDepth int) *EvalCtx {
	evalCtx.evalDepth = evalDepth
	return evalCtx
}

func (evalCtx *EvalCtx) GetPosition() string {
	return evalCtx.position.String()
}

func getSystemMem() uint64 {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		panic("Unable to determine system memory")
	}

	return uint64(in.Totalram) * uint64(in.Unit)
}

func (evalCtx *EvalCtx) InitEngine() {
	evalCtx.g = evalCtx.loadPgnOrFEN()
	if evalCtx.fen != "" {
		evalCtx.position = evalCtx.g.Position()
	} else {
		var halfMoveIndex uint
		halfMoveIndex = (evalCtx.moveNum - 1) * 2
		if evalCtx.turn == chess.Black {
			halfMoveIndex++
		}

		p := evalCtx.g.Positions()
		if halfMoveIndex >= uint(len(p)) {
			panic("bogus move num")
		}
		evalCtx.position = p[halfMoveIndex]
	}

	evalCtx.earlyInitEngine()

	// actual init is deferred until first use as it is exensive and unneeded
	// when we get a cache hit
	evalCtx.doLazyInit = true
}

// just renice and grab the version; full init occurs in lazyInitEngine()
func (evalCtx *EvalCtx) earlyInitEngine() {
	err := evalCtx.engine.Renice()
	if err != nil {
		panic(err)
	}

	err = evalCtx.engine.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame)
	if err != nil {
		panic(err)
	}

	engineVer := evalCtx.engine.ID()["name"]
	engineVerParts := strings.Split(engineVer, " ")
	if len(engineVerParts) < 2 {
		panic("Cannot find stockfish version number")
	}
	evalCtx.engVersion, err = strconv.ParseFloat(engineVerParts[1], 64)
	if err != nil {
		panic("Cannot parse stockfish version number")
	}

}

func (evalCtx *EvalCtx) lazyInitEngine() {
	err := evalCtx.engine.Run(uci.CmdSetOption{Name: "UCI_Chess960",
		Value: "true"})
	if err != nil {
		panic(err)
	}
	err = evalCtx.engine.Run(uci.CmdSetOption{Name: "UCI_ShowWDL",
		Value: "true"})
	if err != nil {
		panic(err)
	}
	err = evalCtx.engine.Run(uci.CmdSetOption{Name: "Threads", Value: strconv.FormatUint(evalCtx.numThreads, 10)})
	if err != nil {
		panic(err)
	}
	err = evalCtx.engine.Run(uci.CmdSetOption{Name: "Hash", Value: strconv.FormatUint(evalCtx.hashSizeInMiB, 10)})
	if err != nil {
		panic(err)
	}
	err = evalCtx.engine.Run(uci.CmdSetOption{Name: "UCI_AnalyseMode", Value: "true"})
	if err != nil {
		panic(err)
	}
	err = evalCtx.engine.Run(uci.CmdSetOption{Name: "Ponder", Value: "true"})
	if err != nil {
		panic(err)
	}
	//err = evalCtx.engine.Run(uci.CmdSetOption{Name: "MultiPV", Value: "5"})

	err = evalCtx.engine.Run(uci.CmdPosition{Position: evalCtx.position})
	if err != nil {
		panic(err)
	}

	evalCtx.doLazyInit = false
}

func (evalCtx *EvalCtx) SetFEN(fen string) *EvalCtx {
	evalCtx.fen = fen
	fenCheck, err := chess.FEN(fen)
	if err != nil {
		panic(err)
	}
	evalCtx.g = chess.NewGame(fenCheck)
	evalCtx.position = evalCtx.g.Position()
	err = evalCtx.engine.Run(uci.CmdPosition{Position: evalCtx.position})
	if err != nil {
		panic(err)
	}

	return evalCtx
}

func (evalCtx *EvalCtx) loadPgnOrFEN() *chess.Game {
	if evalCtx.fen == "" && evalCtx.pgnFile == "" {
		evalCtx.fen = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	}
	if evalCtx.fen != "" {
		fen, err := chess.FEN(evalCtx.fen)
		if err != nil {
			panic(err)
		}
		return chess.NewGame(fen)
	} // else

	readCloser, err := OpenPgn(evalCtx.pgnFile)
	if err != nil {
		panic(err)
	}
	defer readCloser.Close()

	var opts chess.ScannerOpts
	opts.ExpandVariations = false

	scanner := chess.NewScannerWithOptions(readCloser, opts)
	var ret *chess.Game

	for scanner.Scan() {
		ret = scanner.Next()

		// only process 1st game
		break
	}

	err = scanner.Err()
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil {
		panic(err)
	}

	return ret
}

func (evalCtx *EvalCtx) loadResultFromLocalCache(
	staleOk bool) (*EvalResult, error) {

	fen := evalCtx.position.XFENString()
	var err error
	fen, err = NormalizeFEN(fen)
	if err != nil {
		return nil, err
	}
	cacheFileName := fen2CacheFileName(fen)
	cacheFilePath := fen2CacheFilePath(fen)
	cacheFileFullName := filepath.Join(cacheFilePath, cacheFileName)

	encodedResult, err := ioutil.ReadFile(cacheFileFullName)
	if os.IsNotExist(err) {
		fen = evalCtx.position.XFENString()
		cacheFileName = fen2CacheFileName(fen)
		cacheFilePath = fen2CacheFilePath(fen)
		cacheFileFullName = filepath.Join(cacheFilePath, cacheFileName)
		encodedResult, err = ioutil.ReadFile(cacheFileFullName)
	}
	if os.IsNotExist(err) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		panic(err)
	}

	var er EvalResult
	err = json.Unmarshal(encodedResult, &er)
	if err != nil {
		panic(err)
	}

	if evalCtx.engVersion == UnknownEngVer {
		panic("Unknown current engine version")
	}
	if !staleOk && evalCtx.engVersion > er.EngVersion {
		return nil, ErrCacheStale
	}

	er.KNPS = er.KNPS + " (local cache)"
	er.Type = EvalTypeLocalStockfish

	return &er, nil
}

type CloudPV struct {
	CP    int    `json:"cp"`
	Mate  int    `json:"mate"`
	Moves string `json:"moves"`
}

type CloudEvalResp struct {
	Error  string    `json:"error"`
	Fen    string    `json:"fen"`
	KNodes int       `json:"knodes"`
	Depth  int       `json:"depth"`
	PVs    []CloudPV `json:"pvs"`
}

/* @todo consider also checking chessdb evals.
 *
 *  https://www.chessdb.cn/cloudbookc_api_en.html
 *
 * curl -G --data-urlencode "action=queryall" --data-urlencode "board=r2qkbnr/pp1n1ppp/2p5/4p3/2BPP1b1/5N2/PPP3PP/RNBQK2R w KQkq - 4 7" http://www.chessdb.cn/cdb.php --output -
 */
func (evalCtx *EvalCtx) loadResultFromCloudCache(
	staleOk bool) (*EvalResult, error) {

	if evalCtx.cloudCache == false {
		return nil, ErrCacheMiss
	}
	const BaseUrl = "https://lichess.org/api/cloud-eval"

	fen := evalCtx.position.XFENString()
	var err error
	fen, err = NormalizeFEN(fen)
	if err != nil {
		return nil, err
	}
	position := url.QueryEscape(fen)
	queryParams := fmt.Sprintf("?fen=%v&multiPv=1&variant=standard", position)
	requestURL, err := url.Parse(BaseUrl + queryParams)
	if err != nil {
		return nil, fmt.Errorf("eval: failed to parse url:%w", err)
	}

	var resp *http.Response
	retryCount := 0

	for {
		req, err := http.NewRequest("GET", requestURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("eval: failed to create request:%w", err)
		}
		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("eval: GET %v failed: %w",
				requestURL.String(), err)
		}
		if resp.StatusCode == 429 {
			// https://lichess.org/page/api-tips says wait a minute
			fmt.Fprintf(os.Stderr, "eval: 429 recv; sleeping 1min retry:%v...",
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
		return nil, fmt.Errorf("eval: failed to read http response: %w", err)
	}

	var cloudResp CloudEvalResp
	err = json.Unmarshal(body, &cloudResp)
	if err != nil {
		return nil, fmt.Errorf("eval: failed to unmarshel json response.\n\terr:%w\n\tcode:%v\n\tbody:%v", err, resp.StatusCode, string(body))
	}

	if cloudResp.Error != "" {
		if cloudResp.Error == "Not found" {
			return nil, ErrCacheMiss
		} // else
		return nil, fmt.Errorf("eval: cloud fetch error: %v", cloudResp.Error)
	}

	var evalResult EvalResult
	evalResult.CP = cloudResp.PVs[0].CP
	evalResult.Mate = cloudResp.PVs[0].Mate

	// WDL unavailable from cloud cache
	evalResult.WinPct = 0.0
	evalResult.DrawPct = 0.0
	evalResult.LossPct = 0.0

	moveList := strings.Split(cloudResp.PVs[0].Moves, " ")
	uciNotation := chess.UCINotation{}
	bestMove, err := uciNotation.Decode(evalCtx.position, moveList[0])
	if err != nil {
		panic(fmt.Sprintf("BUG: could not decode uci str %v: %v",
			moveList[0], err))
	}

	algNotation := chess.AlgebraicNotation{}
	evalResult.BestMove = algNotation.Encode(evalCtx.position, bestMove)
	evalResult.Depth = cloudResp.Depth
	evalResult.EngVersion = UnknownEngVer // not in response
	evalResult.KNPS = fmt.Sprintf("%v (cloud cache)", cloudResp.KNodes)
	evalResult.SearchTimeInSeconds = UnknownSearchTime // not in response
	evalResult.Type = EvalTypeLichess

	if !staleOk && evalCtx.engVersion > evalResult.EngVersion {
		return nil, ErrCacheStale
	}

	return &evalResult, nil
}

func selectBest(local, cloud *EvalResult) *EvalResult {
	if local == nil {
		return cloud
	} else if cloud == nil {
		return local
	} // else both non-nil

	// for now just use the local one
	return local
}

func selectBestErr(err1, err2 error) error {
	if err1 == nil {
		return err2
	} else if err2 == nil {
		return err1
	} // else both non-nil

	if errors.Is(err1, ErrCacheMiss) ||
		(errors.Is(err1, ErrCacheStale) &&
			!errors.Is(err2, ErrCacheMiss)) {
		return err2
	} // else

	return err1
}

func atLeastOneSuccess(err1, err2 error) bool {
	return (err1 == nil || err2 == nil)
}

func (evalCtx *EvalCtx) loadResultFromCache(
	staleOk bool) (*EvalResult, error) {

	localResult, err1 := evalCtx.loadResultFromLocalCache(staleOk)
	cloudResult, err2 := evalCtx.loadResultFromCloudCache(staleOk)
	if !atLeastOneSuccess(err1, err2) {
		return nil, selectBestErr(err1, err2)
	}

	return selectBest(localResult, cloudResult), nil
}

func (evalCtx *EvalCtx) persistResultToCache(er *EvalResult) {
	fen := evalCtx.position.XFENString()
	var err error
	fen, err = NormalizeFEN(fen)
	if err != nil {
		panic(err)
	}
	cacheFileName := fen2CacheFileName(fen)
	cacheFilePath := fen2CacheFilePath(fen)
	cacheFileFullName := filepath.Join(cacheFilePath, cacheFileName)

	_ = os.Remove(cacheFileFullName)
	err = os.MkdirAll(cacheFilePath, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}
	file, err := os.OpenFile(cacheFileFullName, os.O_CREATE|os.O_RDWR|os.O_EXCL,
		0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	encodedResult, err := json.Marshal(er)
	if err != nil {
		panic(err)
	}

	_, err = file.Write(encodedResult)
	if err != nil {
		panic(err)
	}

	return
}

func fen2CacheFileName(fen string) string {
	fileName := strings.ReplaceAll(fen, "/", "@@@")
	fileName = strings.ReplaceAll(fileName, " ", "___")
	fileName = fmt.Sprintf("%v%v", FileNamePrefix, fileName)

	return fileName
}

func cacheFileName2Fen(fileName string) string {
	fen := fileName[len(FileNamePrefix):]
	fen = strings.ReplaceAll(fen, "___", " ")
	fen = strings.ReplaceAll(fen, "@@@", "/")

	var err error
	fen, err = NormalizeFEN(fen)
	if err != nil {
		panic(err)
	}

	return fen
}

func fen2CacheFilePath(cacheFileName string) string {
	xsum := crc32.ChecksumIEEE([]byte(cacheFileName))
	return fmt.Sprintf("%v/%02x/%02x/%02x/%02x", CacheFileDir, xsum>>24,
		(xsum>>16)&0xff, (xsum>>8)&0xff, xsum&0xff)
}

func (evalCtx *EvalCtx) Eval() *EvalResult {
	fromCache := false
	er, err := evalCtx.loadResultFromCache(evalCtx.staleOk)
	if err == nil {
		fromCache = true

		if evalCtx.cacheOnly ||
			(evalCtx.evalDepth != DefaultDepth && er.Depth >= evalCtx.evalDepth) ||
			(evalCtx.evalDepth == DefaultDepth && uint(math.Round(er.SearchTimeInSeconds)) >= evalCtx.evalTimeInSec) {

			return er
		}
	} else if evalCtx.cacheOnly {
		return nil
	}

	if evalCtx.doLazyInit {
		evalCtx.lazyInitEngine()
	}

	fmt.Fprintf(os.Stderr, "eval: scoring position:%v\n", evalCtx.position)

	searchStartTime := time.Now()

	if evalCtx.evalDepth != DefaultDepth {
		err = evalCtx.engine.Run(uci.CmdGo{Depth: evalCtx.evalDepth})
	} else {
		err = evalCtx.engine.Run(uci.CmdGo{MoveTime: time.Second *
			time.Duration(evalCtx.evalTimeInSec)})
	}
	if err != nil {
		panic(err)
	}

	results := evalCtx.engine.SearchResults()

	searchEndTime := time.Now()

	if fromCache && results.Info.Depth < er.Depth {
		// we had a cached result, searched with the engine anyway, and
		// found a result with a weaker depth than what we had found in
		// the cache. in this scenario, throw away the engine's result and
		// keep the existing cached result. this can occur when the user
		// requested a search by time and there wasn't enough time to find
		// a move that exceeded the depth of the cached entry

		return er
	}

	// results.BestMove doesn't include correct tags, so do this encode/decode
	// dance
	algNotation := chess.AlgebraicNotation{}
	uciNotation := chess.UCINotation{}

	bestMvUciStr := uciNotation.Encode(evalCtx.position, results.BestMove)
	bestMoveFixed, err := uciNotation.Decode(evalCtx.position, bestMvUciStr)
	if err != nil {
		panic(fmt.Sprintf("BUG: could not re-encode decoded uci str %v: %v",
			bestMvUciStr, err))
	}
	winPct, _ := results.Info.Score.WinPct()
	lossPct, _ := results.Info.Score.LossPct()
	drawPct, _ := results.Info.Score.DrawPct()
	er = &EvalResult{
		CP:                  results.Info.Score.CP,
		Mate:                results.Info.Score.Mate,
		WinPct:              winPct,
		LossPct:             lossPct,
		DrawPct:             drawPct,
		BestMove:            algNotation.Encode(evalCtx.position, bestMoveFixed),
		Depth:               results.Info.Depth,
		KNPS:                fmt.Sprintf("%v", results.Info.NPS/1000),
		EngVersion:          evalCtx.engVersion,
		SearchTimeInSeconds: searchEndTime.Sub(searchStartTime).Seconds(),
		Type:                EvalTypeLocalStockfish,
	}

	if evalCtx.g.Position().Turn() == chess.Black {
		er.CP = -er.CP
		er.Mate = -er.Mate
	}

	evalCtx.persistResultToCache(er)

	return er
}

type cachedEvalEntryList struct {
	entries []string
}

// filepath.Walk() doesn't work with symlinks so walk ourselves
func findAllCacheEvalFiles(dirPath string) (cachedEvalEntryList, error) {

	curList := cachedEvalEntryList{}
	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Printf("Failed to open dir %v: %v\n", dirPath, err)
		return curList, err
	}
	defer dir.Close()

	fileList, err := dir.ReadDir(-1)
	if err != nil {
		fmt.Printf("Failed to read dir %v: %v\n", dirPath, err)
		return curList, err
	}

	for _, file := range fileList {
		if file.IsDir() {
			childList, err := findAllCacheEvalFiles(filepath.Join(dirPath, file.Name()))
			if err != nil {
				return curList, err
			}
			curList.entries = append(curList.entries, childList.entries...)
		} else if strings.HasPrefix(file.Name(), FileNamePrefix) {
			curList.entries = append(curList.entries, filepath.Join(dirPath, file.Name()))
		}
	}

	return curList, nil
}

func (evalCtx *EvalCtx) UpgradeCache() error {
	entryList, err := findAllCacheEvalFiles(CacheFileDir)
	if err != nil {
		fmt.Printf("Failed to find all cached evals: %v", err)
		return err
	}

	numEntries := len(entryList.entries)
	fmt.Printf("Checking %v cached entries...\n", numEntries)

	ii := 0
	for _, entry := range entryList.entries {
		ii++
		cacheFile := filepath.Base(entry)
		fen := cacheFileName2Fen(cacheFile)

		evalCtx.SetFEN(fen)
		er, err := evalCtx.loadResultFromLocalCache(true)
		if err != nil {
			return err
		}
		if er.EngVersion == evalCtx.engVersion {
			continue
		}
		evalCtx.staleOk = false
		evalCtx.evalDepth = DefaultDepth
		evalCtx.evalTimeInSec = uint(math.Round(er.SearchTimeInSeconds))
		if evalCtx.evalTimeInSec == UnknownSearchTime {
			evalCtx.evalTimeInSec = DefaultEvalTimeInSec
		}
		fmt.Printf("  Upgrading(%v of %v) fen:%v...\n", ii, numEntries, fen)
		newEr := evalCtx.Eval()
		if er.BestMove != newEr.BestMove {
			fmt.Printf("    *** best move changed from %v(ver %v) to %v(ver %v)\n",
				er.BestMove, er.EngVersion, newEr.BestMove, newEr.EngVersion)
		}
	}

	return nil
}
