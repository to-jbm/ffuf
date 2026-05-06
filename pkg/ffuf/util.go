package ffuf

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"
)

// used for random string generation in calibration function
var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString returns a random string of length of parameter n
func RandomString(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

// UniqStringSlice returns an unordered slice of unique strings. The duplicates are dropped
func UniqStringSlice(inslice []string) []string {
	found := map[string]bool{}

	for _, v := range inslice {
		found[v] = true
	}
	ret := []string{}
	for k := range found {
		ret = append(ret, k)
	}
	return ret
}

// FileExists checks if the filepath exists and is not a directory.
// Returns false in case it's not possible to describe the named file.
func FileExists(path string) bool {
	md, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !md.IsDir()
}

// RequestContainsKeyword checks if a keyword is present in any field of a request
func RequestContainsKeyword(req Request, kw string) bool {
	if strings.Contains(req.Host, kw) {
		return true
	}
	if strings.Contains(req.Url, kw) {
		return true
	}
	if strings.Contains(req.Method, kw) {
		return true
	}
	if strings.Contains(string(req.Data), kw) {
		return true
	}
	for k, v := range req.Headers {
		if strings.Contains(k, kw) || strings.Contains(v, kw) {
			return true
		}
	}
	return false
}

// HostURLFromRequest gets a host + path without the filename or last part of the URL path
func HostURLFromRequest(req Request) string {
	u, _ := url.Parse(req.Url)
	u.Host = req.Host
	pathparts := strings.Split(u.Path, "/")
	trimpath := strings.TrimSpace(strings.Join(pathparts[:len(pathparts)-1], "/"))
	return u.Host + trimpath
}

// Version returns the ffuf version string
func Version() string {
	return fmt.Sprintf("%s%s", VERSION, VERSION_APPENDIX)
}

func CheckOrCreateConfigDir() error {
	var err error
	err = createConfigDir(CONFIGDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(HISTORYDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(SCRAPERDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(AUTOCALIBDIR)
	if err != nil {
		return err
	}
	err = setupDefaultAutocalibrationStrategies()
	return err
}

func createConfigDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		var pError *os.PathError
		if errors.As(err, &pError) {
			return os.MkdirAll(path, 0750)
		}
		return err
	}
	return nil
}

func StrInSlice(key string, slice []string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}
	return false
}

// SlugifyURL turns a URL into a filesystem-safe slug. It strips the scheme,
// replaces every non-alphanumeric run with a single underscore, trims leading
// and trailing underscores, and caps the length at 100 characters. If the
// input slugifies to an empty string, "ffuf" is returned.
func SlugifyURL(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i != -1 {
		s = s[i+3:]
	}
	var b strings.Builder
	b.Grow(len(s))
	prevUnderscore := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore {
				b.WriteRune('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "ffuf"
	}
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

// FormatStartTime formats a time as a filesystem-safe timestamp suitable for
// embedding in default output filenames. Format is YYYY-MM-DD_HHMMSS (no
// colons, so it is safe on Windows filesystems).
func FormatStartTime(t time.Time) string {
	return t.Format("2006-01-02_150405")
}

// AutoOutputFilename returns the default value for -o when the user has not
// provided one. The extension is chosen based on -of:
//   - "all"  -> no extension (writeToAll appends .json/.ejson/.html/...)
//   - other  -> "." + format (e.g. ".json")
func AutoOutputFilename(rawURL string, t time.Time, format string) string {
	base := SlugifyURL(rawURL) + "_" + FormatStartTime(t)
	switch format {
	case "", "all":
		return base
	default:
		return base + "." + format
	}
}

// AutoDebugLogFilename returns the default value for -debug-log when the user
// has not provided one.
func AutoDebugLogFilename(rawURL string, t time.Time) string {
	return SlugifyURL(rawURL) + "_errors_" + FormatStartTime(t) + ".log"
}

func mergeMaps(m1 map[string][]string, m2 map[string][]string) map[string][]string {
	merged := make(map[string][]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		if _, ok := merged[key]; !ok {
			// Key not found, add it
			merged[key] = value
			continue
		}
		for _, entry := range value {
			if !StrInSlice(entry, merged[key]) {
				merged[key] = append(merged[key], entry)
			}
		}
	}
	return merged
}
