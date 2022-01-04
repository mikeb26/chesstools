package chesstools

import (
	"fmt"
	//	"golang.org/x/oauth2/clientcredentials"
	"io"
	"net/http"
	"os"
	"strings"
)

const LichessUrlPrefix = "https://lichess.org"

func OpenPgn(pgnFileOrUrl string) (io.ReadCloser, error) {
	if strings.HasPrefix(pgnFileOrUrl, LichessUrlPrefix) {
		return openPgnLichess(pgnFileOrUrl)
	} // else

	return openPgnFile(pgnFileOrUrl)
}

func openPgnFile(filename string) (io.ReadCloser, error) {
	f, err := os.OpenFile(filename, os.O_RDONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("Failed to open pgn file %v: %w", filename, err)
	}

	return f, nil
}

func openPgnLichess(url string) (io.ReadCloser, error) {
	// e.g. https://lichess.org/1mIMQ8xz for a game
	// or https://lichess.org/study/p1SdJUis for a study
	// https://lichess.org/api#tag/Games says gameId is 8 characters.
	// when clicking from https://lichess.org/@/<user>/all there seems
	// to be 4 additional characters appended, so strip these if present

	const GameIdLen = 8
	const GameIdFieldNum = 4
	const StudyIdLen = 8
	const StudyIdFieldNum = 5
	const LichessUrlGamePath = "/game/export/"
	const LichessUrlStudyPath = "/study/"
	const LichessUrlStudyPrefix = LichessUrlPrefix + LichessUrlStudyPath
	const LichessUrlGameSuffixParams = "?evals=0&clocks=0"
	const LichessUrlStudySuffix = ".pgn"

	var url2Fetch string

	// @todo detect study chapter urls
	if strings.HasPrefix(url, LichessUrlStudyPrefix) {
		urlParts := strings.Split(url, "/")
		if len(urlParts) < StudyIdFieldNum {
			return nil, fmt.Errorf("Cannot find lichess study id field in url %v", url)
		}
		studyId := urlParts[StudyIdFieldNum-1]
		if len(studyId) < StudyIdLen {
			return nil, fmt.Errorf("Malformed study id field in url %v", url)
		}
		if len(studyId) > StudyIdLen {
			studyId = studyId[0:StudyIdLen]
		}
		url2Fetch = LichessUrlStudyPrefix + studyId + LichessUrlStudySuffix
	} else {
		urlParts := strings.Split(url, "/")
		if len(urlParts) < GameIdFieldNum {
			return nil, fmt.Errorf("Cannot find lichess game id field in url %v", url)
		}
		gameId := urlParts[GameIdFieldNum-1]
		if len(gameId) < GameIdLen {
			return nil, fmt.Errorf("Malformed game id field in url %v", url)
		}
		if len(gameId) > GameIdLen {
			gameId = gameId[0:GameIdLen]
		}
		url2Fetch = LichessUrlPrefix + LichessUrlGamePath + gameId + LichessUrlGameSuffixParams
	}

	//client := conf.Client()
	client := &http.Client{}
	req, err := http.NewRequest("GET", url2Fetch, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to construct http request for url %v: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch url %v: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Bad http status attempting to fetch url %v: %v/%v", url, resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}
