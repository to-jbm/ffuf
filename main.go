package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/to-jbm/ffuf/v2/pkg/ffuf"
	"github.com/to-jbm/ffuf/v2/pkg/filter"
	"github.com/to-jbm/ffuf/v2/pkg/input"
	"github.com/to-jbm/ffuf/v2/pkg/interactive"
	"github.com/to-jbm/ffuf/v2/pkg/output"
	"github.com/to-jbm/ffuf/v2/pkg/runner"
	"github.com/to-jbm/ffuf/v2/pkg/scraper"
)

type multiStringFlag []string
type wordlistFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *wordlistFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (m *wordlistFlag) Set(value string) error {
	delimited := strings.Split(value, ",")

	if len(delimited) > 1 {
		*m = append(*m, delimited...)
	} else {
		*m = append(*m, value)
	}

	return nil
}

// ParseFlags parses the command line flags and (re)populates the ConfigOptions struct
func ParseFlags(opts *ffuf.ConfigOptions) *ffuf.ConfigOptions {
	var ignored bool

	var cookies, autocalibrationstrings, autocalibrationstrategies, headers, inputcommands multiStringFlag
	var wordlists, encoders wordlistFlag

	cookies = opts.HTTP.Cookies
	autocalibrationstrings = opts.General.AutoCalibrationStrings
	headers = opts.HTTP.Headers
	inputcommands = opts.Input.Inputcommands
	wordlists = opts.Input.Wordlists
	encoders = opts.Input.Encoders

	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "k", false, "Dummy flag for backwards compatibility")
	flag.BoolVar(&opts.Output.OutputSkipEmptyFile, "or", opts.Output.OutputSkipEmptyFile, "Don't create the output file if we don't have results")
	flag.BoolVar(&opts.General.AutoCalibration, "ac", opts.General.AutoCalibration, "Automatically calibrate filtering options")
	flag.BoolVar(&opts.General.AutoCalibrationPerHost, "ach", opts.General.AutoCalibration, "Per host autocalibration")
	flag.BoolVar(&opts.General.Colors, "c", opts.General.Colors, "Colorize output.")
	flag.BoolVar(&opts.General.Json, "json", opts.General.Json, "JSON output, printing newline-delimited JSON records")
	flag.BoolVar(&opts.General.Noninteractive, "noninteractive", opts.General.Noninteractive, "Disable the interactive console functionality")
	flag.BoolVar(&opts.General.Quiet, "s", opts.General.Quiet, "Do not print additional information (silent mode)")
	flag.BoolVar(&opts.General.ShowVersion, "V", opts.General.ShowVersion, "Show version information.")
	flag.BoolVar(&opts.General.StopOn403, "sf", opts.General.StopOn403, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&opts.General.StopOnAll, "sa", opts.General.StopOnAll, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&opts.General.StopOnErrors, "se", opts.General.StopOnErrors, "Stop on spurious errors")
	flag.BoolVar(&opts.General.Verbose, "v", opts.General.Verbose, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.BoolVar(&opts.HTTP.FollowRedirects, "r", opts.HTTP.FollowRedirects, "Follow redirects")
	flag.BoolVar(&opts.HTTP.IgnoreBody, "ignore-body", opts.HTTP.IgnoreBody, "Do not fetch the response content.")
	flag.BoolVar(&opts.HTTP.Raw, "raw", opts.HTTP.Raw, "Do not encode URI")
	flag.BoolVar(&opts.HTTP.Recursion, "recursion", opts.HTTP.Recursion, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.BoolVar(&opts.HTTP.Http2, "http2", opts.HTTP.Http2, "Use HTTP2 protocol")
	flag.BoolVar(&opts.Input.DirSearchCompat, "D", opts.Input.DirSearchCompat, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.BoolVar(&opts.Input.IgnoreWordlistComments, "ic", opts.Input.IgnoreWordlistComments, "Ignore wordlist comments")
	flag.IntVar(&opts.General.MaxTime, "maxtime", opts.General.MaxTime, "Maximum running time in seconds for entire process.")
	flag.IntVar(&opts.General.MaxTimeJob, "maxtime-job", opts.General.MaxTimeJob, "Maximum running time in seconds per job.")
	flag.IntVar(&opts.General.Rate, "rate", opts.General.Rate, "Rate of requests per second")
	flag.IntVar(&opts.General.Threads, "t", opts.General.Threads, "Number of concurrent threads.")
	flag.IntVar(&opts.HTTP.RecursionDepth, "recursion-depth", opts.HTTP.RecursionDepth, "Maximum recursion depth.")
	flag.IntVar(&opts.HTTP.Timeout, "timeout", opts.HTTP.Timeout, "HTTP request timeout in seconds.")
	flag.IntVar(&opts.Input.InputNum, "input-num", opts.Input.InputNum, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.StringVar(&opts.General.AutoCalibrationKeyword, "ack", opts.General.AutoCalibrationKeyword, "Autocalibration keyword")
	flag.StringVar(&opts.HTTP.ClientCert, "cc", "", "Client cert for authentication. Client key needs to be defined as well for this to work")
	flag.StringVar(&opts.HTTP.ClientKey, "ck", "", "Client key for authentication. Client certificate needs to be defined as well for this to work")
	flag.StringVar(&opts.General.ConfigFile, "config", "", "Load configuration from a file")
	flag.StringVar(&opts.General.ScraperFile, "scraperfile", "", "Custom scraper file path")
	flag.StringVar(&opts.General.Scrapers, "scrapers", opts.General.Scrapers, "Active scraper groups")
	flag.StringVar(&opts.Filter.Mode, "fmode", opts.Filter.Mode, "Filter set operator. Either of: and, or")
	flag.StringVar(&opts.Filter.Lines, "fl", opts.Filter.Lines, "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&opts.Filter.Regexp, "fr", opts.Filter.Regexp, "Filter regexp")
	flag.StringVar(&opts.Filter.Size, "fs", opts.Filter.Size, "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.Filter.Status, "fc", opts.Filter.Status, "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.Filter.Time, "ft", opts.Filter.Time, "Filter by number of milliseconds to the first response byte, either greater or less than. EG: >100 or <100")
	flag.StringVar(&opts.Filter.Words, "fw", opts.Filter.Words, "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.General.Delay, "p", opts.General.Delay, "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.General.Searchhash, "search", opts.General.Searchhash, "Search for a FFUFHASH payload from ffuf history")
	flag.StringVar(&opts.HTTP.Data, "d", opts.HTTP.Data, "POST data")
	flag.StringVar(&opts.HTTP.Data, "data", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-ascii", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-binary", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Method, "X", opts.HTTP.Method, "HTTP method to use")
	flag.StringVar(&opts.HTTP.ProxyURL, "x", opts.HTTP.ProxyURL, "Proxy URL (SOCKS5 or HTTP). For example: http://127.0.0.1:8080 or socks5://127.0.0.1:8080")
	flag.StringVar(&opts.General.ProxiesFile, "proxies", opts.General.ProxiesFile, "File containing proxy URLs for round-robin rotation (one per line). Each request uses the next proxy in sequence. Overrides -x.")
	flag.StringVar(&opts.HTTP.ReplayProxyURL, "replay-proxy", opts.HTTP.ReplayProxyURL, "Replay matched requests using this proxy.")
	flag.StringVar(&opts.HTTP.RecursionStrategy, "recursion-strategy", opts.HTTP.RecursionStrategy, "Recursion strategy: \"default\" for a redirect based, and \"greedy\" to recurse on all matches")
	flag.StringVar(&opts.HTTP.URL, "u", opts.HTTP.URL, "Target URL")
	flag.StringVar(&opts.HTTP.SNI, "sni", opts.HTTP.SNI, "Target TLS SNI, does not support FUZZ keyword")
	flag.StringVar(&opts.Input.Extensions, "e", opts.Input.Extensions, "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.StringVar(&opts.Input.InputMode, "mode", opts.Input.InputMode, "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork, sniper")
	flag.StringVar(&opts.Input.InputShell, "input-shell", opts.Input.InputShell, "Shell to be used for running command")
	flag.StringVar(&opts.Input.Request, "request", opts.Input.Request, "File containing the raw http request")
	flag.StringVar(&opts.Input.RequestProto, "request-proto", opts.Input.RequestProto, "Protocol to use along with raw request")
	flag.StringVar(&opts.Matcher.Mode, "mmode", opts.Matcher.Mode, "Matcher set operator. Either of: and, or")
	flag.StringVar(&opts.Matcher.Lines, "ml", opts.Matcher.Lines, "Match amount of lines in response")
	flag.StringVar(&opts.Matcher.Regexp, "mr", opts.Matcher.Regexp, "Match regexp")
	flag.StringVar(&opts.Matcher.Size, "ms", opts.Matcher.Size, "Match HTTP response size")
	flag.StringVar(&opts.Matcher.Status, "mc", opts.Matcher.Status, "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.Matcher.Time, "mt", opts.Matcher.Time, "Match how many milliseconds to the first response byte, either greater or less than. EG: >100 or <100")
	flag.StringVar(&opts.Matcher.Words, "mw", opts.Matcher.Words, "Match amount of words in response")
	flag.StringVar(&opts.General.WAFCodes, "wmc", opts.General.WAFCodes, "WAF/rate-limit detector: HTTP status codes treated as a WAF/rate-limit response. Comma separated list and ranges. Default 403,429,502,504 (set to \"\" to disable WAF detection entirely)")
	flag.StringVar(&opts.General.WAFSize, "wms", opts.General.WAFSize, "WAF/rate-limit detector: response size(s) treated as a WAF/rate-limit response")
	flag.StringVar(&opts.General.WAFWords, "wmw", opts.General.WAFWords, "WAF/rate-limit detector: response word count(s) treated as a WAF/rate-limit response")
	flag.StringVar(&opts.General.WAFLines, "wml", opts.General.WAFLines, "WAF/rate-limit detector: response line count(s) treated as a WAF/rate-limit response")
	flag.StringVar(&opts.General.WAFRegexp, "wmr", opts.General.WAFRegexp, "WAF/rate-limit detector: regexp matched against response (headers + body) treated as a WAF/rate-limit response")
	flag.StringVar(&opts.General.WAFTime, "wtime", opts.General.WAFTime, "WAF/rate-limit backoff ladder in seconds (comma separated). e.g. 30,60,120 - escalates per consecutive trigger, stays at last value afterwards. Setting this implicitly engages WAF detection with default codes 403,429,401")
	flag.IntVar(&opts.General.WAFThreshold, "wthreshold", opts.General.WAFThreshold, "WAF/rate-limit detector: number of consecutive WAF/rate-limit responses required to trigger a backoff (and consecutive non-WAF responses needed to reset the ladder).")
	flag.StringVar(&opts.Output.AuditLog, "audit-log", opts.Output.AuditLog, "Write audit log containing all requests, responses and config")
	flag.StringVar(&opts.Output.DebugLog, "debug-log", opts.Output.DebugLog, "Write all of the internal logging to the specified file.")
	flag.StringVar(&opts.Output.OutputDirectory, "od", opts.Output.OutputDirectory, "Directory path to store matched results to.")
	flag.StringVar(&opts.Output.OutputFile, "o", opts.Output.OutputFile, "Write output to file")
	flag.StringVar(&opts.Output.OutputFormat, "of", opts.Output.OutputFormat, "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.Var(&autocalibrationstrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.Var(&autocalibrationstrategies, "acs", "Custom auto-calibration strategies. Can be used multiple times. Implies -ac")
	flag.Var(&cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	flag.Var(&cookies, "cookie", "Cookie data (alias of -b)")
	flag.Var(&headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.Var(&inputcommands, "input-cmd", "Command producing the input. --input-num is required when using this input method. Overrides -w.")
	flag.Var(&wordlists, "w", "Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'")
	flag.Var(&encoders, "enc", "Encoders for keywords, eg. 'FUZZ:urlencode b64encode'")
	flag.Usage = Usage
	flag.Parse()

	opts.General.AutoCalibrationStrings = autocalibrationstrings
	if len(autocalibrationstrategies) > 0 {
		opts.General.AutoCalibrationStrategies = []string{}
		for _, strategy := range autocalibrationstrategies {
			opts.General.AutoCalibrationStrategies = append(opts.General.AutoCalibrationStrategies, strings.Split(strategy, ",")...)
		}
	}
	opts.HTTP.Cookies = cookies
	opts.HTTP.Headers = headers
	opts.Input.Inputcommands = inputcommands
	opts.Input.Wordlists = wordlists
	opts.Input.Encoders = encoders
	return opts
}

func main() {

	var err, optserr error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Capture a single start-time used for any auto-generated default
	// filenames (output, debug log) so that they remain stable across the
	// lifetime of the process (pause / interactive / resume reuse them).
	startTime := time.Now()
	// Capture the full shell pipeline as early as possible so sibling
	// processes (e.g. `cat | tail | grep | ffuf -w -`) are still alive
	// when we walk them. Stored on Config below and embedded into the
	// output file alongside CommandLine.
	fullCommand := ffuf.CaptureFullCommand()
	// prepare the default config options from default config file
	var opts *ffuf.ConfigOptions
	opts, optserr = ffuf.ReadDefaultConfig()

	opts = ParseFlags(opts)

	// Handle searchhash functionality and exit
	if opts.General.Searchhash != "" {
		coptions, pos, err := ffuf.SearchHash(opts.General.Searchhash)
		if err != nil {
			fmt.Printf("[ERR] %s\n", err)
			os.Exit(1)
		}
		if len(coptions) > 0 {
			fmt.Printf("Request candidate(s) for hash %s\n", opts.General.Searchhash)
		}
		for _, copt := range coptions {
			conf, err := ffuf.ConfigFromOptions(&copt.ConfigOptions, ctx, cancel)
			if err != nil {
				continue
			}
			ok, reason := ffuf.HistoryReplayable(conf)
			if ok {
				printSearchResults(conf, pos, copt.Time, opts.General.Searchhash)
			} else {
				fmt.Printf("[ERR] Hash cannot be mapped back because %s\n", reason)
			}

		}
		if err != nil {
			fmt.Printf("[ERR] %s\n", err)
		}
		os.Exit(0)
	}

	if opts.General.ShowVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.Version())
		os.Exit(0)
	}

	if opts.General.ConfigFile != "" {
		opts, err = ffuf.ReadConfig(opts.General.ConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Encoutered error(s): %s\n", err)
			Usage()
			fmt.Fprintf(os.Stderr, "Encoutered error(s): %s\n", err)
			os.Exit(1)
		}
		// Reset the flag package state
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		// Re-parse the cli options
		opts = ParseFlags(opts)
	}

	autoDebugLog := opts.Output.DebugLog == ""
	autoOutputFile := opts.Output.OutputFile == ""
	if autoDebugLog || autoOutputFile {
		if err := os.MkdirAll(ffuf.DefaultOutputDir, 0750); err != nil {
			fmt.Fprintf(os.Stderr, "Could not create default output directory %q: %s\n", ffuf.DefaultOutputDir, err)
			os.Exit(1)
		}
		// Copy dashboard panel.py to output directory for user convenience
		copyPanelToOutputDir()
	}

	// If the user did not supply -debug-log, derive a default name from
	// the URL slug and start time so error logs are always captured.
	// This must run AFTER potential config-file / -request handling has
	// populated opts.HTTP.URL. We fall back to a blank URL slug if none
	// was provided yet (e.g. -request-only configs); ConfigFromOptions
	// will report missing -u in that case anyway.
	if autoDebugLog {
		opts.Output.DebugLog = ffuf.AutoDebugLogFilename(opts.HTTP.URL, startTime)
	}
	if len(opts.Output.DebugLog) != 0 {
		f, err := os.OpenFile(opts.Output.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Disabling logging, encountered error(s): %s\n", err)
			log.SetOutput(io.Discard)
		} else {
			log.SetOutput(f)
			defer f.Close()
		}
	} else {
		log.SetOutput(io.Discard)
	}
	if optserr != nil {
		log.Printf("Error while opening default config file: %s", optserr)
	}

	// Set up Config struct
	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}

	// Pin the start time on the config so downstream code (output writer,
	// auto-naming, audit log) all share the same clock anchor.
	conf.StartTime = startTime
	conf.Debuglog = opts.Output.DebugLog
	conf.FullCommand = fullCommand

	// If the user did not supply -o, derive a default output filename from
	// the URL slug and start time. We do this after ConfigFromOptions so
	// the URL is final (raw-request mode populates Url from the file). The
	// filename is generated once and stays the same across pause / resume
	// / interactive mode so the same file is overwritten on each periodic
	// save. ConfigFromOptions only copies OutputFormat into conf when
	// OutputFile is non-empty, so we also have to mirror it here in the
	// auto case (default "json").
	if autoOutputFile {
		fmtOut := opts.Output.OutputFormat
		if fmtOut == "" {
			fmtOut = "json"
		}
		validFormats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}
		found := false
		for _, f := range validFormats {
			if f == fmtOut {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Unknown output file format (-of): %s, defaulting to json\n", fmtOut)
			fmtOut = "json"
		}
		conf.OutputFormat = fmtOut
		conf.OutputFile = ffuf.AutoOutputFilename(conf.Url, startTime, conf.OutputFormat)
	}

	job, err := prepareJob(conf)

	if job.AuditLogger != nil {
		defer job.AuditLogger.Close()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}
	if err := SetupFilters(opts, conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}

	if !conf.Noninteractive {
		go func() {
			err := interactive.Handle(job)
			if err != nil {
				log.Printf("Error while trying to initialize interactive session: %s", err)
			}
		}()
	}

	// Job handles waiting for goroutines to complete itself
	job.Start()
}

func prepareJob(conf *ffuf.Config) (*ffuf.Job, error) {
	var err error
	job := ffuf.NewJob(conf)
	var errs ffuf.Multierror
	job.Input, errs = input.NewInputProvider(conf)
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	job.Runner = runner.NewRunnerByName("http", conf, false)
	if len(conf.ReplayProxyURL) > 0 {
		job.ReplayRunner = runner.NewRunnerByName("http", conf, true)
	}
	// We only have stdout outputprovider right now
	job.Output = output.NewOutputProviderByName("stdout", conf)

	// Initialize the audit logger if specified
	if len(conf.AuditLog) > 0 {
		job.AuditLogger, err = output.NewAuditLogger(conf.AuditLog)
		if err != nil {
			errs.Add(err)
		} else {
			err = job.AuditLogger.Write(conf)
			if err != nil {
				errs.Add(err)
			}
		}
	}

	// Initialize scraper
	newscraper, scraper_err := scraper.FromDir(ffuf.SCRAPERDIR, conf.Scrapers)
	if scraper_err.ErrorOrNil() != nil {
		errs.Add(scraper_err.ErrorOrNil())
	}
	job.Scraper = newscraper
	if conf.ScraperFile != "" {
		err = job.Scraper.AppendFromFile(conf.ScraperFile)
		if err != nil {
			errs.Add(err)
		}
	}
	return job, errs.ErrorOrNil()
}

func SetupFilters(parseOpts *ffuf.ConfigOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	conf.MatcherManager = filter.NewMatcherManager()
	// If any other matcher is set, ignore -mc default value
	matcherSet := false
	statusSet := false
	warningIgnoreBody := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "mc" {
			statusSet = true
		}
		if f.Name == "ms" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "ml" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "mr" {
			matcherSet = true
		}
		if f.Name == "mt" {
			matcherSet = true
		}
		if f.Name == "mw" {
			matcherSet = true
			warningIgnoreBody = true
		}
	})
	// Only set default matchers if no
	if statusSet || !matcherSet {
		if err := conf.MatcherManager.AddMatcher("status", parseOpts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if parseOpts.Filter.Status != "" {
		if err := conf.MatcherManager.AddFilter("status", parseOpts.Filter.Status, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Size != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("size", parseOpts.Filter.Size, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Regexp != "" {
		if err := conf.MatcherManager.AddFilter("regexp", parseOpts.Filter.Regexp, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Words != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("word", parseOpts.Filter.Words, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Lines != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("line", parseOpts.Filter.Lines, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Time != "" {
		if err := conf.MatcherManager.AddFilter("time", parseOpts.Filter.Time, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Size != "" {
		if err := conf.MatcherManager.AddMatcher("size", parseOpts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Regexp != "" {
		if err := conf.MatcherManager.AddMatcher("regexp", parseOpts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Words != "" {
		if err := conf.MatcherManager.AddMatcher("word", parseOpts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Lines != "" {
		if err := conf.MatcherManager.AddMatcher("line", parseOpts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Time != "" {
		if err := conf.MatcherManager.AddMatcher("time", parseOpts.Matcher.Time); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		fmt.Printf("*** Warning: possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw.\n")
	}

	// WAF / rate-limit detector matchers (independent from result matchers)
	if conf.WAFMatchers == nil {
		conf.WAFMatchers = make(map[string]ffuf.FilterProvider)
	}
	type wafMatcherSpec struct {
		name  string
		flag  string
		value string
	}
	wafSpecs := []wafMatcherSpec{
		{"status", "wmc", parseOpts.General.WAFCodes},
		{"size", "wms", parseOpts.General.WAFSize},
		{"word", "wmw", parseOpts.General.WAFWords},
		{"line", "wml", parseOpts.General.WAFLines},
		{"regexp", "wmr", parseOpts.General.WAFRegexp},
	}
	for _, spec := range wafSpecs {
		if spec.value == "" {
			continue
		}
		fp, ferr := filter.NewFilterByName(spec.name, spec.value)
		if ferr != nil {
			errs.Add(fmt.Errorf("WAF detector (-%s): %s", spec.flag, ferr.Error()))
			continue
		}
		conf.WAFMatchers[spec.name] = fp
	}
	if len(conf.WAFMatchers) > 0 && len(conf.WAFTimes) == 0 {
		// Should be handled in ConfigFromOptions, but be defensive
		conf.WAFTimes = []int{30, 60, 120}
	}
	return errs.ErrorOrNil()
}

func printSearchResults(conf *ffuf.Config, pos int, exectime time.Time, hash string) {
	inp, err := input.NewInputProvider(conf)
	if err.ErrorOrNil() != nil {
		fmt.Printf("-------------------------------------------\n")
		fmt.Println("Encountered error that prevents reproduction of the request:")
		fmt.Println(err.ErrorOrNil())
		return
	}
	inp.SetPosition(pos)
	inputdata := inp.Value()
	inputdata["FFUFHASH"] = []byte(hash)
	basereq := ffuf.BaseRequest(conf)
	dummyrunner := runner.NewRunnerByName("simple", conf, false)
	ffufreq, _ := dummyrunner.Prepare(inputdata, &basereq)
	rawreq, _ := dummyrunner.Dump(&ffufreq)
	fmt.Printf("-------------------------------------------\n")
	fmt.Printf("ffuf job started at: %s\n\n", exectime.Format(time.RFC3339))
	fmt.Printf("%s\n", string(rawreq))
}

// copyPanelToOutputDir copies panel.py from the ffuf binary directory to the
// output directory (ffuf_out/) so users can easily launch the web dashboard.
// If the source file doesn't exist or copy fails, the error is logged but
// does not stop the scan.
func copyPanelToOutputDir() {
	// Determine source: look next to the binary, then in working directory
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	sourceDir := filepath.Dir(exePath)
	sourceFile := filepath.Join(sourceDir, "panel.py")

	// Fallback to current working directory if not found next to binary
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		wd, _ := os.Getwd()
		sourceFile = filepath.Join(wd, "panel.py")
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			return // panel.py not found, skip silently
		}
	}

	destFile := filepath.Join(ffuf.DefaultOutputDir, "panel.py")

	// Read source
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		log.Printf("Could not read panel.py: %s", err)
		return
	}

	// Write destination (overwrite if exists to ensure latest version)
	if err := os.WriteFile(destFile, data, 0755); err != nil {
		log.Printf("Could not copy panel.py to output dir: %s", err)
		return
	}

	log.Printf("Dashboard copied to: %s", destFile)
}
