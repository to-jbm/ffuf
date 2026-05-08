package output

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/to-jbm/ffuf/v2/pkg/ffuf"
)

const (
	BANNER_HEADER = `
        /'___\  /'___\           /'___\       
       /\ \__/ /\ \__/  __  __  /\ \__/       
       \ \ ,__\\ \ ,__\/\ \/\ \ \ \ ,__\      
        \ \ \_/ \ \ \_/\ \ \_\ \ \ \ \_/      
         \ \_\   \ \_\  \ \____/  \ \_\       
          \/_/    \/_/   \/___/    \/_/       
`
	BANNER_SEP = "________________________________________________"
)

type Stdoutput struct {
	config         *ffuf.Config
	fuzzkeywords   []string
	Results        []ffuf.Result
	CurrentResults []ffuf.Result
	// resultsMu guards Results and CurrentResults so the periodic save
	// goroutine in Job can safely snapshot them while worker goroutines
	// are still appending new results.
	resultsMu sync.Mutex
	// fileMu serializes actual writes to the output file so two SaveFile
	// invocations cannot race each other on disk (e.g. interactive savejson
	// happening in the middle of a periodic save).
	fileMu sync.Mutex
	// pendingMu guards pendingPrints. Results recorded while the WAF
	// backoff is active are stashed here and printed in order on the
	// first call to FlushPendingResults() after the pause ends.
	pendingMu     sync.Mutex
	pendingPrints []ffuf.Result
}

func NewStdoutput(conf *ffuf.Config) *Stdoutput {
	var outp Stdoutput
	outp.config = conf
	outp.Results = make([]ffuf.Result, 0)
	outp.CurrentResults = make([]ffuf.Result, 0)
	outp.fuzzkeywords = make([]string, 0)
	for _, ip := range conf.InputProviders {
		outp.fuzzkeywords = append(outp.fuzzkeywords, ip.Keyword)
	}
	sort.Strings(outp.fuzzkeywords)
	return &outp
}

func (s *Stdoutput) Banner() {
	version := strings.ReplaceAll(ffuf.Version(), "<3", fmt.Sprintf("%s<3%s", ANSI_RED, ANSI_CLEAR))
	fmt.Fprintf(os.Stderr, "%s\n       v%s\n%s\n\n", BANNER_HEADER, version, BANNER_SEP)
	printOption([]byte("Method"), []byte(s.config.Method))
	printOption([]byte("URL"), []byte(s.config.Url))

	// Print wordlists
	for _, provider := range s.config.InputProviders {
		if provider.Name == "wordlist" {
			printOption([]byte("Wordlist"), []byte(provider.Keyword+": "+provider.Value))
		}
	}

	// Print headers
	if len(s.config.Headers) > 0 {
		for k, v := range s.config.Headers {
			printOption([]byte("Header"), []byte(fmt.Sprintf("%s: %s", k, v)))
		}
	}
	// Print POST data
	if len(s.config.Data) > 0 {
		printOption([]byte("Data"), []byte(s.config.Data))
	}

	// Print extensions
	if len(s.config.Extensions) > 0 {
		exts := ""
		for _, ext := range s.config.Extensions {
			exts = fmt.Sprintf("%s%s ", exts, ext)
		}
		printOption([]byte("Extensions"), []byte(exts))
	}

	// Output file info
	if len(s.config.OutputFile) > 0 {

		// Use filename as specified by user
		OutputFile := s.config.OutputFile

		if s.config.OutputFormat == "all" {
			// Actually... append all extensions
			OutputFile += ".{json,ejson,html,md,csv,ecsv}"
		}

		printOption([]byte("Output file"), []byte(OutputFile))
		printOption([]byte("File format"), []byte(s.config.OutputFormat))
	}

	// Follow redirects?
	follow := fmt.Sprintf("%t", s.config.FollowRedirects)
	printOption([]byte("Follow redirects"), []byte(follow))

	// Autocalibration
	autocalib := fmt.Sprintf("%t", s.config.AutoCalibration)
	printOption([]byte("Calibration"), []byte(autocalib))

	// Proxies
	if len(s.config.ProxyURL) > 0 {
		printOption([]byte("Proxy"), []byte(s.config.ProxyURL))
	}
	if len(s.config.ReplayProxyURL) > 0 {
		printOption([]byte("ReplayProxy"), []byte(s.config.ReplayProxyURL))
	}

	// Timeout
	timeout := fmt.Sprintf("%d", s.config.Timeout)
	printOption([]byte("Timeout"), []byte(timeout))

	// Threads
	threads := fmt.Sprintf("%d", s.config.Threads)
	printOption([]byte("Threads"), []byte(threads))

	// Delay?
	if s.config.Delay.HasDelay {
		delay := ""
		if s.config.Delay.IsRange {
			delay = fmt.Sprintf("%.2f - %.2f seconds", s.config.Delay.Min, s.config.Delay.Max)
		} else {
			delay = fmt.Sprintf("%.2f seconds", s.config.Delay.Min)
		}
		printOption([]byte("Delay"), []byte(delay))
	}

	// Print matchers
	for _, f := range s.config.MatcherManager.GetMatchers() {
		printOption([]byte("Matcher"), []byte(f.ReprVerbose()))
	}
	// Print filters
	for _, f := range s.config.MatcherManager.GetFilters() {
		printOption([]byte("Filter"), []byte(f.ReprVerbose()))
	}
	fmt.Fprintf(os.Stderr, "%s\n\n", BANNER_SEP)
}

// Reset resets the result slice
func (s *Stdoutput) Reset() {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.CurrentResults = make([]ffuf.Result, 0)
}

// Cycle moves the CurrentResults to Results and resets the results slice
func (s *Stdoutput) Cycle() {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.Results = append(s.Results, s.CurrentResults...)
	s.CurrentResults = make([]ffuf.Result, 0)
}

// GetResults returns the result slice
func (s *Stdoutput) GetCurrentResults() []ffuf.Result {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	out := make([]ffuf.Result, len(s.CurrentResults))
	copy(out, s.CurrentResults)
	return out
}

// SetResults sets the result slice
func (s *Stdoutput) SetCurrentResults(results []ffuf.Result) {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	s.CurrentResults = results
}

// snapshotResults returns a defensive copy of the combined Results +
// CurrentResults slices. Caller must NOT hold resultsMu.
func (s *Stdoutput) snapshotResults() []ffuf.Result {
	s.resultsMu.Lock()
	defer s.resultsMu.Unlock()
	out := make([]ffuf.Result, 0, len(s.Results)+len(s.CurrentResults))
	out = append(out, s.Results...)
	out = append(out, s.CurrentResults...)
	return out
}

func (s *Stdoutput) Progress(status ffuf.Progress) {
	if s.config.Quiet {
		// No progress for quiet mode
		return
	}

	dur := time.Since(status.StartedAt)
	runningSecs := int(dur / time.Second)
	var reqRate int64
	if runningSecs > 0 {
		reqRate = status.ReqSec
	} else {
		reqRate = 0
	}

	hours := dur / time.Hour
	dur -= hours * time.Hour
	mins := dur / time.Minute
	dur -= mins * time.Minute
	secs := dur / time.Second

	fmt.Fprintf(os.Stderr, "%s:: Progress: [%d/%d] :: Job [%d/%d] :: %d req/sec :: Duration: [%d:%02d:%02d] :: Errors: %d ::", TERMINAL_CLEAR_LINE, status.ReqCount, status.ReqTotal, status.QueuePos, status.QueueTotal, reqRate, hours, mins, secs, status.ErrorCount)
}

func (s *Stdoutput) Info(infostring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", infostring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[INFO] %s\n\n", TERMINAL_CLEAR_LINE, infostring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sINFO%s] %s\n\n", TERMINAL_CLEAR_LINE, ANSI_BLUE, ANSI_CLEAR, infostring)
		}
	}
}

func (s *Stdoutput) Error(errstring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", errstring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[ERR]Â %s\n", TERMINAL_CLEAR_LINE, errstring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sERR%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, errstring)
		}
	}
}

func (s *Stdoutput) Warning(warnstring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", warnstring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[WARN] %s\n", TERMINAL_CLEAR_LINE, warnstring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sWARN%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, warnstring)
		}
	}
}

func (s *Stdoutput) Raw(output string) {
	fmt.Fprintf(os.Stderr, "%s%s", TERMINAL_CLEAR_LINE, output)
}

func (s *Stdoutput) writeToAll(filename string, config *ffuf.Config, res []ffuf.Result) error {
	var err error
	var BaseFilename string = s.config.OutputFile

	// Go through each type of write, adding
	// the suffix to each output file.

	s.config.OutputFile = BaseFilename + ".json"
	err = writeJSON(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ejson"
	err = writeEJSON(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".html"
	err = writeHTML(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".md"
	err = writeMarkdown(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".csv"
	err = writeCSV(s.config.OutputFile, s.config, res, false)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ecsv"
	err = writeCSV(s.config.OutputFile, s.config, res, true)
	if err != nil {
		s.Error(err.Error())
	}

	return nil

}

// SaveFile saves the current results to a file of a given type. Safe to be
// called concurrently with worker Result() appends and with itself; writes
// are serialized by fileMu so the on-disk file is never half-written.
func (s *Stdoutput) SaveFile(filename, format string) error {
	combined := s.snapshotResults()
	if s.config.OutputSkipEmptyFile && len(combined) == 0 {
		// During a periodic save loop we silently skip the empty-file case;
		// the user only sees this notice on Finalize via the explicit info
		// line in Stop.
		return nil
	}

	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	var err error
	switch format {
	case "all":
		err = s.writeToAll(filename, s.config, combined)
	case "json":
		err = writeJSON(filename, s.config, combined)
	case "ejson":
		err = writeEJSON(filename, s.config, combined)
	case "html":
		err = writeHTML(filename, s.config, combined)
	case "md":
		err = writeMarkdown(filename, s.config, combined)
	case "csv":
		err = writeCSV(filename, s.config, combined, false)
	case "ecsv":
		err = writeCSV(filename, s.config, combined, true)
	}
	return err
}

// Finalize gets run after all the ffuf jobs are completed
func (s *Stdoutput) Finalize() error {
	var err error
	if s.config.OutputFile != "" {
		err = s.SaveFile(s.config.OutputFile, s.config.OutputFormat)
		if err != nil {
			s.Error(err.Error())
		}
	}
	if !s.config.Quiet {
		fmt.Fprintf(os.Stderr, "\n")
	}
	return nil
}

func (s *Stdoutput) Result(resp ffuf.Response) {
	// Do we want to write request and response to a file
	if len(s.config.OutputDirectory) > 0 {
		resp.ResultFile = s.writeResultToFile(resp)
	}

	inputs := make(map[string][]byte, len(resp.Request.Input))
	for k, v := range resp.Request.Input {
		inputs[k] = v
	}
	sResult := ffuf.Result{
		Input:            inputs,
		Position:         resp.Request.Position,
		StatusCode:       resp.StatusCode,
		ContentLength:    resp.ContentLength,
		ContentWords:     resp.ContentWords,
		ContentLines:     resp.ContentLines,
		ContentType:      resp.ContentType,
		RedirectLocation: resp.GetRedirectLocation(false),
		ScraperData:      resp.ScraperData,
		Url:              resp.Request.Url,
		Duration:         resp.Duration,
		ResultFile:       resp.ResultFile,
		Host:             resp.Request.Host,
		Proxy:            resp.Proxy,
	}
	s.resultsMu.Lock()
	s.CurrentResults = append(s.CurrentResults, sResult)
	s.resultsMu.Unlock()

	// If we are currently in a WAF/rate-limit backoff sleep, defer the
	// stdout print so the pause msg stays clean. The buffered results
	// are flushed in order by FlushPendingResults() once the pause ends.
	if s.config.IsWAFBackingOff() {
		s.pendingMu.Lock()
		s.pendingPrints = append(s.pendingPrints, sResult)
		s.pendingMu.Unlock()
		return
	}
	s.PrintResult(sResult)
}

// FlushPendingResults prints (in arrival order) every result that was
// recorded while a WAF backoff was active. After this call the pending
// buffer is empty. Safe to call when no results are pending; it is also
// safe to call concurrently with new appends because we drain the slice
// under the mutex and print outside of it.
func (s *Stdoutput) FlushPendingResults() {
	s.pendingMu.Lock()
	if len(s.pendingPrints) == 0 {
		s.pendingMu.Unlock()
		return
	}
	pending := s.pendingPrints
	s.pendingPrints = nil
	s.pendingMu.Unlock()
	for _, r := range pending {
		s.PrintResult(r)
	}
}

func (s *Stdoutput) writeResultToFile(resp ffuf.Response) string {
	var fileContent, fileName, filePath string
	// Create directory if needed
	if s.config.OutputDirectory != "" {
		err := os.MkdirAll(s.config.OutputDirectory, 0750)
		if err != nil {
			if !os.IsExist(err) {
				s.Error(err.Error())
				return ""
			}
		}
	}
	fileContent = fmt.Sprintf("%s\n---- â†‘ Request ---- Response â†“ ----\n\n%s", resp.Request.Raw, resp.Raw)

	// Create file name
	fileName = fmt.Sprintf("%x", md5.Sum([]byte(fileContent)))

	filePath = path.Join(s.config.OutputDirectory, fileName)
	err := os.WriteFile(filePath, []byte(fileContent), 0640)
	if err != nil {
		s.Error(err.Error())
	}
	return fileName
}

func (s *Stdoutput) PrintResult(res ffuf.Result) {
	switch {
	case s.config.Json:
		s.resultJson(res)
	case s.config.Quiet:
		s.resultQuiet(res)
	case len(s.fuzzkeywords) > 1 || s.config.Verbose || len(s.config.OutputDirectory) > 0 || len(res.ScraperData) > 0:
		// Print a multi-line result (when using multiple input keywords and wordlists)
		s.resultMultiline(res)
	default:
		s.resultNormal(res)
	}
}

func (s *Stdoutput) prepareInputsOneLine(res ffuf.Result) string {
	inputs := ""
	if len(s.fuzzkeywords) > 1 {
		for _, k := range s.fuzzkeywords {
			if ffuf.StrInSlice(k, s.config.CommandKeywords) {
				// If we're using external command for input, display the position instead of input
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, strconv.Itoa(res.Position))
			} else {
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, res.Input[k])
			}
		}
	} else {
		for _, k := range s.fuzzkeywords {
			if ffuf.StrInSlice(k, s.config.CommandKeywords) {
				// If we're using external command for input, display the position instead of input
				inputs = strconv.Itoa(res.Position)
			} else {
				inputs = string(res.Input[k])
			}
		}
	}
	return inputs
}

func (s *Stdoutput) resultQuiet(res ffuf.Result) {
	fmt.Println(s.prepareInputsOneLine(res))
}

func stripProxyCredentials(proxyURL string) string {
	if proxyURL == "" {
		return ""
	}
	// Parse URL and return scheme://host:port only (no user:pass)
	if idx := strings.LastIndex(proxyURL, "://"); idx != -1 {
		scheme := proxyURL[:idx+3]
		remainder := proxyURL[idx+3:]
		// Remove user:pass@ if present
		if atIdx := strings.LastIndex(remainder, "@"); atIdx != -1 {
			return scheme + remainder[atIdx+1:]
		}
	}
	return proxyURL
}

func (s *Stdoutput) resultMultiline(res ffuf.Result) {
	var res_hdr, res_str string
	res_str = "%s%s    * %s: %s\n"
	res_hdr = fmt.Sprintf("%s%s[Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms]%s", TERMINAL_CLEAR_LINE, s.colorize(res.StatusCode), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), ANSI_CLEAR)
	reslines := ""
	if s.config.Verbose {
		reslines = fmt.Sprintf("%s%s| URL | %s\n", reslines, TERMINAL_CLEAR_LINE, res.Url)
		redirectLocation := res.RedirectLocation
		if redirectLocation != "" {
			reslines = fmt.Sprintf("%s%s| --> | %s\n", reslines, TERMINAL_CLEAR_LINE, redirectLocation)
		}
	}
	if res.ResultFile != "" {
		reslines = fmt.Sprintf("%s%s| RES | %s\n", reslines, TERMINAL_CLEAR_LINE, res.ResultFile)
	}
	if res.Proxy != "" {
		reslines = fmt.Sprintf("%s%s| PROXY | %s\n", reslines, TERMINAL_CLEAR_LINE, stripProxyCredentials(res.Proxy))
	}
	for _, k := range s.fuzzkeywords {
		if ffuf.StrInSlice(k, s.config.CommandKeywords) {
			// If we're using external command for input, display the position instead of input
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, strconv.Itoa(res.Position))
		} else {
			// Wordlist input
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, res.Input[k])
		}
	}
	if len(res.ScraperData) > 0 {
		reslines = fmt.Sprintf("%s%s| SCR |\n", reslines, TERMINAL_CLEAR_LINE)
		for k, vslice := range res.ScraperData {
			for _, v := range vslice {
				reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, v)
			}
		}
	}
	fmt.Printf("%s\n%s\n", res_hdr, reslines)
}

func (s *Stdoutput) resultNormal(res ffuf.Result) {
	proxyInfo := ""
	if res.Proxy != "" {
		proxyInfo = fmt.Sprintf(" [Proxy: %s]", stripProxyCredentials(res.Proxy))
	}
	resnormal := fmt.Sprintf("%s%s%-23s [Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms]%s%s", TERMINAL_CLEAR_LINE, s.colorize(res.StatusCode), s.prepareInputsOneLine(res), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), ANSI_CLEAR, proxyInfo)
	fmt.Println(resnormal)
}

func (s *Stdoutput) resultJson(res ffuf.Result) {
	resBytes, err := json.Marshal(res)
	if err != nil {
		s.Error(err.Error())
	} else {
		fmt.Fprint(os.Stderr, TERMINAL_CLEAR_LINE)
		fmt.Println(string(resBytes))
	}
}

func (s *Stdoutput) colorize(status int64) string {
	if !s.config.Colors {
		return ""
	}
	colorCode := ANSI_CLEAR
	if status >= 200 && status < 300 {
		colorCode = ANSI_GREEN
	}
	if status >= 300 && status < 400 {
		colorCode = ANSI_BLUE
	}
	if status >= 400 && status < 500 {
		colorCode = ANSI_YELLOW
	}
	if status >= 500 && status < 600 {
		colorCode = ANSI_RED
	}
	return colorCode
}

func printOption(name []byte, value []byte) {
	fmt.Fprintf(os.Stderr, " :: %-16s : %s\n", name, value)
}
