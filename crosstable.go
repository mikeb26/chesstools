package chesstools

import (
	_ "embed"

	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	LichessApiBaseUrl = "https://lichess.org/api/crosstable"
)

type CrossTableGames struct {
	NumGames int `json:"nbGames"`
}

func GetCrossTable(player1 string, player2 string) (int, error) {

	var requestURL *url.URL
	var err error

	requestURL, err = url.Parse(LichessApiBaseUrl + "/" + player1 + "/" + player2)
	if err != nil {
		return 0, fmt.Errorf("crosstable: failed to parse url:%w", err)
	}

	var resp *http.Response
	retryCount := 0

	for {
		req, err := http.NewRequest("GET", requestURL.String(), nil)
		if err != nil {
			return 0, fmt.Errorf("opening: failed to create request:%w", err)
		}

		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("crosstable: GET %v failed: %w",
				requestURL.String(), err)
		}
		if resp.StatusCode == 429 {
			// https://lichess.org/page/api-tips says wait a minute
			fmt.Fprintf(os.Stderr, "crosstable: 429 recv; sleeping 1min retry:%v...",
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
		return 0, fmt.Errorf("crosstable: failed to read http response: %w", err)
	}

	var crossTable CrossTableGames
	err = json.Unmarshal(body, &crossTable)
	if err != nil {
		return 0, fmt.Errorf("crosstable: failed to unmarshal json response.\n\turl:%v\n\terr:%w\n\tcode:%v\n\tbody:%v",
			requestURL.String(), err, resp.StatusCode, string(body))
	}

	return crossTable.NumGames, nil
}
