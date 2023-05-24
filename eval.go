package chesstools

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
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
)

var ErrCacheMiss = errors.New("cache miss")
var ErrCacheStale = errors.New("cache stale")

type EvalResult struct {
	CP         int
	Mate       int
	BestMove   string
	Depth      int
	EngVersion float64
	KNPS       string
}

type EvalCtx struct {
	turn          chess.Color
	moveNum       uint
	pgnFile       string
	fen           string
	numThreads    uint64 // default == num CPU hyperthreads
	hashSizeInMiB uint64 // default == 75% system RAM
	evalTimeInSec uint   // default == 5 minutes
	evalDepth     int    // default == infinite
	g             *chess.Game
	cacheOnly     bool

	engine     *uci.Engine
	engVersion float64
	position   *chess.Position
}

func (evalCtx *EvalCtx) Close() {
	evalCtx.engine.Close()
	evalCtx.engine = nil
}
func NewEvalCtx() *EvalCtx {
	rv := &EvalCtx{}

	rv.turn = chess.White
	rv.moveNum = 0
	rv.pgnFile = ""
	rv.fen = ""
	rv.numThreads = uint64(runtime.NumCPU())
	rv.hashSizeInMiB = (getSystemMem() * 3) / (MiB * 4)
	rv.evalTimeInSec = DefaultEvalTimeInSec
	rv.evalDepth = DefaultDepth
	rv.g = nil
	rv.position = nil
	rv.cacheOnly = false

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

func (evalCtx *EvalCtx) WithCacheOnly(cacheOnly bool) *EvalCtx {
	evalCtx.cacheOnly = cacheOnly
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
	err = evalCtx.engine.Run(uci.CmdPosition{Position: evalCtx.position})
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}

	return ret
}

func (evalCtx *EvalCtx) loadResultFromLocalCache(
	staleOk bool) (*EvalResult, error) {

	cacheFileName := fen2CacheFileName(evalCtx.position.String())
	cacheFilePath := fen2CacheFilePath(evalCtx.position.String())
	cacheFileFullName := filepath.Join(cacheFilePath, cacheFileName)

	encodedResult, err := ioutil.ReadFile(cacheFileFullName)
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

	if !staleOk && evalCtx.engVersion > er.EngVersion {
		return nil, ErrCacheStale
	}

	er.KNPS = er.KNPS + " (local cache)"

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

func (evalCtx *EvalCtx) loadResultFromCloudCache(
	staleOk bool) (*EvalResult, error) {

	const BaseUrl = "https://lichess.org/api/cloud-eval"

	position := url.QueryEscape(evalCtx.position.String())
	queryParams := fmt.Sprintf("?fen=%v", position)
	requestURL, err := url.Parse(BaseUrl + queryParams)
	if err != nil {
		return nil, fmt.Errorf("eval: failed to parse url:%w", err)
	}

	var resp *http.Response
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
	moveList := strings.Split(cloudResp.PVs[0].Moves, " ")
	evalResult.BestMove = moveList[0]
	evalResult.Depth = cloudResp.Depth
	evalResult.EngVersion = 14.1 // not in response; hardcode for now
	evalResult.KNPS = fmt.Sprintf("%v (cloud cache)", cloudResp.KNodes)

	if !staleOk && evalCtx.engVersion > evalResult.EngVersion {
		return nil, ErrCacheStale
	}

	return &evalResult, nil
}

func selectBest(r1, r2 *EvalResult) *EvalResult {
	if r1 == nil {
		return r2
	} else if r2 == nil {
		return r1
	} // else both non-nil

	if r1.Depth > r2.Depth {
		return r1
	} else if r2.Depth > r1.Depth {
		return r2
	} // else both have equal depths

	if r1.KNPS > r2.KNPS {
		return r1
	} else if r2.KNPS > r1.KNPS {
		return r2
	} // else both have equal KNPS

	if r1.EngVersion > r2.EngVersion {
		return r1
	} // else

	return r2
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
	cacheFileName := fen2CacheFileName(evalCtx.position.String())
	cacheFilePath := fen2CacheFilePath(evalCtx.position.String())
	cacheFileFullName := filepath.Join(cacheFilePath, cacheFileName)

	_ = os.Remove(cacheFileFullName)
	err := os.MkdirAll(cacheFilePath, 0755)
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
	fileName = fmt.Sprintf("fen.%v", fileName)

	return fileName
}

func fen2CacheFilePath(cacheFileName string) string {
	xsum := crc32.ChecksumIEEE([]byte(cacheFileName))
	return fmt.Sprintf("cache/%02x/%02x/%02x/%02x", xsum>>24, (xsum>>16)&0xff,
		(xsum>>8)&0xff, xsum&0xff)
}

func (evalCtx *EvalCtx) Eval() *EvalResult {
	fromCache := false
	er, err := evalCtx.loadResultFromCache(evalCtx.cacheOnly)
	if err == nil {
		fromCache = true

		if evalCtx.cacheOnly || (evalCtx.evalDepth != DefaultDepth &&
			er.Depth >= evalCtx.evalDepth) {
			// we were asked to search by depth, have a cache hit, and the cached
			// entry has a greater depth than was requested so we can use it
			return er
		}
	} else if evalCtx.cacheOnly {
		return nil
	}

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

	if fromCache && results.Info.Depth < er.Depth {
		// we had a cached result, searched with the engine anyway, and
		// found a result with a weaker depth than what we had found in
		// the cache. in this scenario, throw away the engine's result and
		// keep the existing cached result. this can occur when the user
		// requested a search by time and there wasn't enough time to find
		// a move that exceeded the depth of the cached entry

		return er
	}

	//var notation chess.AlgebraicNotation
	er = &EvalResult{
		CP:   results.Info.Score.CP,
		Mate: results.Info.Score.Mate,
		//BestMove: notation.Encode(evalCtx.position, results.BestMove)
		BestMove:   results.BestMove.String(),
		Depth:      results.Info.Depth,
		KNPS:       fmt.Sprintf("%v", results.Info.NPS/1000),
		EngVersion: evalCtx.engVersion,
	}

	if evalCtx.g.Position().Turn() == chess.Black {
		er.CP = -er.CP
		er.Mate = -er.Mate
	}

	evalCtx.persistResultToCache(er)

	return er
}
