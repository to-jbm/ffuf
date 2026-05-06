package ffuf

import (
	"context"
	"sync/atomic"
	"time"
)

type Config struct {
	AuditLog                  string                `json:"auditlog"`
	AutoCalibration           bool                  `json:"autocalibration"`
	AutoCalibrationKeyword    string                `json:"autocalibration_keyword"`
	AutoCalibrationPerHost    bool                  `json:"autocalibration_perhost"`
	AutoCalibrationStrategies []string              `json:"autocalibration_strategies"`
	AutoCalibrationStrings    []string              `json:"autocalibration_strings"`
	Cancel                    context.CancelFunc    `json:"-"`
	Colors                    bool                  `json:"colors"`
	CommandKeywords           []string              `json:"-"`
	CommandLine               string                `json:"cmdline"`
	ConfigFile                string                `json:"configfile"`
	Context                   context.Context       `json:"-"`
	Data                      string                `json:"postdata"`
	Debuglog                  string                `json:"debuglog"`
	Delay                     optRange              `json:"delay"`
	DirSearchCompat           bool                  `json:"dirsearch_compatibility"`
	Encoders                  []string              `json:"encoders"`
	Extensions                []string              `json:"extensions"`
	FilterMode                string                `json:"fmode"`
	FollowRedirects           bool                  `json:"follow_redirects"`
	Headers                   map[string]string     `json:"headers"`
	IgnoreBody                bool                  `json:"ignorebody"`
	IgnoreWordlistComments    bool                  `json:"ignore_wordlist_comments"`
	InputMode                 string                `json:"inputmode"`
	InputNum                  int                   `json:"cmd_inputnum"`
	InputProviders            []InputProviderConfig `json:"inputproviders"`
	InputShell                string                `json:"inputshell"`
	Json                      bool                  `json:"json"`
	MatcherManager            MatcherManager        `json:"matchers"`
	MatcherMode               string                `json:"mmode"`
	MaxTime                   int                   `json:"maxtime"`
	MaxTimeJob                int                   `json:"maxtime_job"`
	Method                    string                `json:"method"`
	Noninteractive            bool                  `json:"noninteractive"`
	OutputDirectory           string                `json:"outputdirectory"`
	OutputFile                string                `json:"outputfile"`
	OutputFormat              string                `json:"outputformat"`
	OutputSkipEmptyFile       bool                  `json:"OutputSkipEmptyFile"`
	ProgressFrequency         int                   `json:"-"`
	ProxyURL                  string                `json:"proxyurl"`
	Quiet                     bool                  `json:"quiet"`
	Rate                      int64                 `json:"rate"`
	Raw                       bool                  `json:"raw"`
	Recursion                 bool                  `json:"recursion"`
	RecursionDepth            int                   `json:"recursion_depth"`
	RecursionStrategy         string                `json:"recursion_strategy"`
	ReplayProxyURL            string                `json:"replayproxyurl"`
	RequestFile               string                `json:"requestfile"`
	RequestProto              string                `json:"requestproto"`
	ScraperFile               string                `json:"scraperfile"`
	Scrapers                  string                `json:"scrapers"`
	SNI                       string                `json:"sni"`
	StopOn403                 bool                  `json:"stop_403"`
	StopOnAll                 bool                  `json:"stop_all"`
	StopOnErrors              bool                  `json:"stop_errors"`
	Threads                   int                   `json:"threads"`
	Timeout                   int                   `json:"timeout"`
	Url                       string                `json:"url"`
	Verbose                   bool                  `json:"verbose"`
	Wordlists                 []string              `json:"wordlists"`
	Http2                     bool                  `json:"http2"`
	ClientCert                string                `json:"client-cert"`
	ClientKey                 string                `json:"client-key"`
	// WAF / rate-limit adaptive backoff
	WAFMatchers  map[string]FilterProvider `json:"-"`
	WAFTimes     []int                     `json:"waf_times"`
	WAFThreshold int                       `json:"waf_threshold"`
	// StartTime is the wall-clock time at which the ffuf process was launched.
	// It is used to derive the default output and debug-log filenames so that
	// the same files are reused across pause / interactive / resume.
	StartTime time.Time `json:"start_time"`
	// TotalPositions is the total number of input positions this run will
	// iterate over (set once at job start). 0 means unknown / not yet set.
	TotalPositions int64 `json:"total_positions"`
	// LastProcessedPosition is the maximum input position of any request that
	// has completed (success or HTTP error). Updated atomically by workers and
	// embedded in the output file so the user can see how far the run has
	// progressed and resume from a specific position later.
	LastProcessedPosition int64 `json:"last_position"`
}

// SetLastProcessedPosition atomically updates LastProcessedPosition to the
// max of its current value and pos.
func (c *Config) SetLastProcessedPosition(pos int) {
	p := int64(pos)
	for {
		cur := atomic.LoadInt64(&c.LastProcessedPosition)
		if p <= cur {
			return
		}
		if atomic.CompareAndSwapInt64(&c.LastProcessedPosition, cur, p) {
			return
		}
	}
}

// GetLastProcessedPosition returns LastProcessedPosition with an atomic load.
func (c *Config) GetLastProcessedPosition() int64 {
	return atomic.LoadInt64(&c.LastProcessedPosition)
}

type InputProviderConfig struct {
	Name     string `json:"name"`
	Keyword  string `json:"keyword"`
	Value    string `json:"value"`
	Encoders string `json:"encoders"`
	Template string `json:"template"` // the templating string used for sniper mode (usually "§")
}

func NewConfig(ctx context.Context, cancel context.CancelFunc) Config {
	var conf Config
	conf.AutoCalibrationKeyword = "FUZZ"
	conf.AutoCalibrationStrategies = []string{"basic"}
	conf.AutoCalibrationStrings = make([]string, 0)
	conf.CommandKeywords = make([]string, 0)
	conf.Context = ctx
	conf.Cancel = cancel
	conf.Data = ""
	conf.Debuglog = ""
	conf.Delay = optRange{0, 0, false, false}
	conf.DirSearchCompat = false
	conf.Encoders = make([]string, 0)
	conf.Extensions = make([]string, 0)
	conf.FilterMode = "or"
	conf.FollowRedirects = false
	conf.Headers = make(map[string]string)
	conf.IgnoreWordlistComments = false
	conf.InputMode = "clusterbomb"
	conf.InputNum = 0
	conf.InputShell = ""
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.Json = false
	conf.MatcherMode = "or"
	conf.MaxTime = 0
	conf.MaxTimeJob = 0
	conf.Method = "GET"
	conf.Noninteractive = false
	conf.ProgressFrequency = 125
	conf.ProxyURL = ""
	conf.Quiet = false
	conf.Rate = 0
	conf.Raw = false
	conf.Recursion = false
	conf.RecursionDepth = 0
	conf.RecursionStrategy = "default"
	conf.RequestFile = ""
	conf.RequestProto = "https"
	conf.SNI = ""
	conf.ScraperFile = ""
	conf.Scrapers = "all"
	conf.StopOn403 = false
	conf.StopOnAll = false
	conf.StopOnErrors = false
	conf.Timeout = 10
	conf.Url = ""
	conf.Verbose = false
	conf.Wordlists = []string{}
	conf.Http2 = false
	conf.WAFMatchers = make(map[string]FilterProvider)
	conf.WAFTimes = []int{}
	conf.WAFThreshold = 10
	conf.StartTime = time.Now()
	conf.TotalPositions = 0
	conf.LastProcessedPosition = 0
	return conf
}

func (c *Config) SetContext(ctx context.Context, cancel context.CancelFunc) {
	c.Context = ctx
	c.Cancel = cancel
}
