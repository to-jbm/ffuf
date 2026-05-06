package output

import (
	"encoding/json"
	"os"
	"time"

	"github.com/to-jbm/ffuf/v2/pkg/ffuf"
)

type ejsonFileOutput struct {
	CommandLine string `json:"commandline"`
	// FullCommand is a best-effort reconstruction of the full shell
	// pipeline (e.g. "cat words.txt | grep foo | ffuf -w -"). May be
	// empty if pipeline reconstruction failed.
	FullCommand string `json:"full_command"`
	Time        string `json:"time"`
	// LastPosition is the highest input position of any completed request
	// at the time the file was written. Combined with TotalPositions it
	// gives an at-a-glance view of run progress and can be used as a
	// resume hint (skip wordlist entries up to this index).
	LastPosition   int64         `json:"last_position"`
	TotalPositions int64         `json:"total_positions"`
	Results        []ffuf.Result `json:"results"`
	Config         *ffuf.Config  `json:"config"`
}

type JsonResult struct {
	Input            map[string]string   `json:"input"`
	Position         int                 `json:"position"`
	StatusCode       int64               `json:"status"`
	ContentLength    int64               `json:"length"`
	ContentWords     int64               `json:"words"`
	ContentLines     int64               `json:"lines"`
	ContentType      string              `json:"content-type"`
	RedirectLocation string              `json:"redirectlocation"`
	ScraperData      map[string][]string `json:"scraper"`
	Duration         time.Duration       `json:"duration"`
	ResultFile       string              `json:"resultfile"`
	Url              string              `json:"url"`
	Host             string              `json:"host"`
}

type jsonFileOutput struct {
	CommandLine    string       `json:"commandline"`
	FullCommand    string       `json:"full_command"`
	Time           string       `json:"time"`
	LastPosition   int64        `json:"last_position"`
	TotalPositions int64        `json:"total_positions"`
	Results        []JsonResult `json:"results"`
	Config         *ffuf.Config `json:"config"`
}

func writeEJSON(filename string, config *ffuf.Config, res []ffuf.Result) error {
	t := time.Now()
	outJSON := ejsonFileOutput{
		CommandLine:    config.CommandLine,
		FullCommand:    config.FullCommand,
		Time:           t.Format(time.RFC3339),
		LastPosition:   config.GetLastProcessedPosition(),
		TotalPositions: config.TotalPositions,
		Results:        res,
	}

	outBytes, err := json.Marshal(outJSON)
	if err != nil {
		return err
	}
	err = os.WriteFile(filename, outBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

func writeJSON(filename string, config *ffuf.Config, res []ffuf.Result) error {
	t := time.Now()
	jsonRes := make([]JsonResult, 0)
	for _, r := range res {
		strinput := make(map[string]string)
		for k, v := range r.Input {
			strinput[k] = string(v)
		}
		jsonRes = append(jsonRes, JsonResult{
			Input:            strinput,
			Position:         r.Position,
			StatusCode:       r.StatusCode,
			ContentLength:    r.ContentLength,
			ContentWords:     r.ContentWords,
			ContentLines:     r.ContentLines,
			ContentType:      r.ContentType,
			RedirectLocation: r.RedirectLocation,
			ScraperData:      r.ScraperData,
			Duration:         r.Duration,
			ResultFile:       r.ResultFile,
			Url:              r.Url,
			Host:             r.Host,
		})
	}
	outJSON := jsonFileOutput{
		CommandLine:    config.CommandLine,
		FullCommand:    config.FullCommand,
		Time:           t.Format(time.RFC3339),
		LastPosition:   config.GetLastProcessedPosition(),
		TotalPositions: config.TotalPositions,
		Results:        jsonRes,
		Config:         config,
	}
	outBytes, err := json.Marshal(outJSON)
	if err != nil {
		return err
	}
	err = os.WriteFile(filename, outBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}
