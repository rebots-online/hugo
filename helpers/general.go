// Copyright 2019 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gohugoio/hugo/common/loggers"

	"github.com/gohugoio/hugo/common/hugo"

	"github.com/spf13/afero"

	"github.com/jdkato/prose/transform"

	bp "github.com/gohugoio/hugo/bufferpool"
	"github.com/spf13/pflag"
)

// FilePathSeparator as defined by os.Separator.
const FilePathSeparator = string(filepath.Separator)

// TCPListen starts listening on a valid TCP port.
func TCPListen() (net.Listener, *net.TCPAddr, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, nil, err
	}
	addr := l.Addr()
	if a, ok := addr.(*net.TCPAddr); ok {
		return l, a, nil
	}
	l.Close()
	return nil, nil, fmt.Errorf("unable to obtain a valid tcp port: %v", addr)

}

// InStringArray checks if a string is an element of a slice of strings
// and returns a boolean value.
func InStringArray(arr []string, el string) bool {
	for _, v := range arr {
		if v == el {
			return true
		}
	}
	return false
}

// FirstUpper returns a string with the first character as upper case.
func FirstUpper(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// UniqueStrings returns a new slice with any duplicates removed.
func UniqueStrings(s []string) []string {
	unique := make([]string, 0, len(s))
	for i, val := range s {
		var seen bool
		for j := 0; j < i; j++ {
			if s[j] == val {
				seen = true
				break
			}
		}
		if !seen {
			unique = append(unique, val)
		}
	}
	return unique
}

// UniqueStringsReuse returns a slice with any duplicates removed.
// It will modify the input slice.
func UniqueStringsReuse(s []string) []string {
	result := s[:0]
	for i, val := range s {
		var seen bool

		for j := 0; j < i; j++ {
			if s[j] == val {
				seen = true
				break
			}
		}

		if !seen {
			result = append(result, val)
		}
	}
	return result
}

// UniqueStringsReuse returns a sorted slice with any duplicates removed.
// It will modify the input slice.
func UniqueStringsSorted(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	ss := sort.StringSlice(s)
	ss.Sort()
	i := 0
	for j := 1; j < len(s); j++ {
		if !ss.Less(i, j) {
			continue
		}
		i++
		s[i] = s[j]
	}

	return s[:i+1]
}

// ReaderToBytes takes an io.Reader argument, reads from it
// and returns bytes.
func ReaderToBytes(lines io.Reader) []byte {
	if lines == nil {
		return []byte{}
	}
	b := bp.GetBuffer()
	defer bp.PutBuffer(b)

	b.ReadFrom(lines)

	bc := make([]byte, b.Len())
	copy(bc, b.Bytes())
	return bc
}

// ReaderToString is the same as ReaderToBytes, but returns a string.
func ReaderToString(lines io.Reader) string {
	if lines == nil {
		return ""
	}
	b := bp.GetBuffer()
	defer bp.PutBuffer(b)
	b.ReadFrom(lines)
	return b.String()
}

// ReaderContains reports whether subslice is within r.
func ReaderContains(r io.Reader, subslice []byte) bool {
	if r == nil || len(subslice) == 0 {
		return false
	}

	bufflen := len(subslice) * 4
	halflen := bufflen / 2
	buff := make([]byte, bufflen)
	var err error
	var n, i int

	for {
		i++
		if i == 1 {
			n, err = io.ReadAtLeast(r, buff[:halflen], halflen)
		} else {
			if i != 2 {
				// shift left to catch overlapping matches
				copy(buff[:], buff[halflen:])
			}
			n, err = io.ReadAtLeast(r, buff[halflen:], halflen)
		}

		if n > 0 && bytes.Contains(buff, subslice) {
			return true
		}

		if err != nil {
			break
		}
	}
	return false
}

// GetTitleFunc returns a func that can be used to transform a string to
// title case.
//
// # The supported styles are
//
// - "Go" (strings.Title)
// - "AP" (see https://www.apstylebook.com/)
// - "Chicago" (see http://www.chicagomanualofstyle.org/home.html)
//
// If an unknown or empty style is provided, AP style is what you get.
func GetTitleFunc(style string) func(s string) string {
	switch strings.ToLower(style) {
	case "go":
		return strings.Title
	case "chicago":
		tc := transform.NewTitleConverter(transform.ChicagoStyle)
		return tc.Title
	default:
		tc := transform.NewTitleConverter(transform.APStyle)
		return tc.Title
	}
}

// HasStringsPrefix tests whether the string slice s begins with prefix slice s.
func HasStringsPrefix(s, prefix []string) bool {
	return len(s) >= len(prefix) && compareStringSlices(s[0:len(prefix)], prefix)
}

// HasStringsSuffix tests whether the string slice s ends with suffix slice s.
func HasStringsSuffix(s, suffix []string) bool {
	return len(s) >= len(suffix) && compareStringSlices(s[len(s)-len(suffix):], suffix)
}

func compareStringSlices(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// DistinctLogger ignores duplicate log statements.
type DistinctLogger struct {
	loggers.Logger
	sync.RWMutex
	m map[string]bool
}

func (l *DistinctLogger) Reset() {
	l.Lock()
	defer l.Unlock()

	l.m = make(map[string]bool)
}

// Println will log the string returned from fmt.Sprintln given the arguments,
// but not if it has been logged before.
func (l *DistinctLogger) Println(v ...any) {
	// fmt.Sprint doesn't add space between string arguments
	logStatement := strings.TrimSpace(fmt.Sprintln(v...))
	l.printIfNotPrinted("println", logStatement, func() {
		l.Logger.Println(logStatement)
	})
}

// Printf will log the string returned from fmt.Sprintf given the arguments,
// but not if it has been logged before.
func (l *DistinctLogger) Printf(format string, v ...any) {
	logStatement := fmt.Sprintf(format, v...)
	l.printIfNotPrinted("printf", logStatement, func() {
		l.Logger.Printf(format, v...)
	})
}

func (l *DistinctLogger) Debugf(format string, v ...any) {
	logStatement := fmt.Sprintf(format, v...)
	l.printIfNotPrinted("debugf", logStatement, func() {
		l.Logger.Debugf(format, v...)
	})
}

func (l *DistinctLogger) Debugln(v ...any) {
	logStatement := fmt.Sprint(v...)
	l.printIfNotPrinted("debugln", logStatement, func() {
		l.Logger.Debugln(v...)
	})
}

func (l *DistinctLogger) Infof(format string, v ...any) {
	logStatement := fmt.Sprintf(format, v...)
	l.printIfNotPrinted("info", logStatement, func() {
		l.Logger.Infof(format, v...)
	})
}

func (l *DistinctLogger) Infoln(v ...any) {
	logStatement := fmt.Sprint(v...)
	l.printIfNotPrinted("infoln", logStatement, func() {
		l.Logger.Infoln(v...)
	})
}

func (l *DistinctLogger) Warnf(format string, v ...any) {
	logStatement := fmt.Sprintf(format, v...)
	l.printIfNotPrinted("warnf", logStatement, func() {
		l.Logger.Warnf(format, v...)
	})
}

func (l *DistinctLogger) Warnln(v ...any) {
	logStatement := fmt.Sprint(v...)
	l.printIfNotPrinted("warnln", logStatement, func() {
		l.Logger.Warnln(v...)
	})
}

func (l *DistinctLogger) Errorf(format string, v ...any) {
	logStatement := fmt.Sprint(v...)
	l.printIfNotPrinted("errorf", logStatement, func() {
		l.Logger.Errorf(format, v...)
	})
}

func (l *DistinctLogger) Errorln(v ...any) {
	logStatement := fmt.Sprint(v...)
	l.printIfNotPrinted("errorln", logStatement, func() {
		l.Logger.Errorln(v...)
	})
}

func (l *DistinctLogger) hasPrinted(key string) bool {
	l.RLock()
	defer l.RUnlock()
	_, found := l.m[key]
	return found
}

func (l *DistinctLogger) printIfNotPrinted(level, logStatement string, print func()) {
	key := level + logStatement
	if l.hasPrinted(key) {
		return
	}
	l.Lock()
	defer l.Unlock()
	l.m[key] = true // Placing this after print() can cause duplicate warning entries to be logged when --panicOnWarning is true.
	print()

}

// NewDistinctErrorLogger creates a new DistinctLogger that logs ERRORs
func NewDistinctErrorLogger() loggers.Logger {
	return &DistinctLogger{m: make(map[string]bool), Logger: loggers.NewErrorLogger()}
}

// NewDistinctLogger creates a new DistinctLogger that logs to the provided logger.
func NewDistinctLogger(logger loggers.Logger) loggers.Logger {
	return &DistinctLogger{m: make(map[string]bool), Logger: logger}
}

// NewDistinctWarnLogger creates a new DistinctLogger that logs WARNs
func NewDistinctWarnLogger() loggers.Logger {
	return &DistinctLogger{m: make(map[string]bool), Logger: loggers.NewWarningLogger()}
}

var (
	// DistinctErrorLog can be used to avoid spamming the logs with errors.
	DistinctErrorLog = NewDistinctErrorLogger()

	// DistinctWarnLog can be used to avoid spamming the logs with warnings.
	DistinctWarnLog = NewDistinctWarnLogger()
)

// InitLoggers resets the global distinct loggers.
func InitLoggers() {
	DistinctErrorLog.Reset()
	DistinctWarnLog.Reset()
}

// Deprecated informs about a deprecation, but only once for a given set of arguments' values.
// If the err flag is enabled, it logs as an ERROR (will exit with -1) and the text will
// point at the next Hugo release.
// The idea is two remove an item in two Hugo releases to give users and theme authors
// plenty of time to fix their templates.
func Deprecated(item, alternative string, err bool) {
	if err {
		DistinctErrorLog.Errorf("%s is deprecated and will be removed in Hugo %s. %s", item, hugo.CurrentVersion.Next().ReleaseVersion(), alternative)
	} else {
		var warnPanicMessage string
		if !loggers.PanicOnWarning.Load() {
			warnPanicMessage = "\n\nRe-run Hugo with the flag --panicOnWarning to get a better error message."
		}
		DistinctWarnLog.Warnf("%s is deprecated and will be removed in a future release. %s%s", item, alternative, warnPanicMessage)
	}
}

// SliceToLower goes through the source slice and lowers all values.
func SliceToLower(s []string) []string {
	if s == nil {
		return nil
	}

	l := make([]string, len(s))
	for i, v := range s {
		l[i] = strings.ToLower(v)
	}

	return l
}

// MD5String takes a string and returns its MD5 hash.
func MD5String(f string) string {
	h := md5.New()
	h.Write([]byte(f))
	return hex.EncodeToString(h.Sum([]byte{}))
}

// MD5FromFileFast creates a MD5 hash from the given file. It only reads parts of
// the file for speed, so don't use it if the files are very subtly different.
// It will not close the file.
func MD5FromFileFast(r io.ReadSeeker) (string, error) {
	const (
		// Do not change once set in stone!
		maxChunks = 8
		peekSize  = 64
		seek      = 2048
	)

	h := md5.New()
	buff := make([]byte, peekSize)

	for i := 0; i < maxChunks; i++ {
		if i > 0 {
			_, err := r.Seek(seek, 0)
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", err
			}
		}

		_, err := io.ReadAtLeast(r, buff, peekSize)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				h.Write(buff)
				break
			}
			return "", err
		}
		h.Write(buff)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// MD5FromReader creates a MD5 hash from the given reader.
func MD5FromReader(r io.Reader) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", nil
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsWhitespace determines if the given rune is whitespace.
func IsWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// NormalizeHugoFlags facilitates transitions of Hugo command-line flags,
// e.g. --baseUrl to --baseURL, --uglyUrls to --uglyURLs
func NormalizeHugoFlags(f *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "baseUrl":
		name = "baseURL"
	case "uglyUrls":
		name = "uglyURLs"
	}
	return pflag.NormalizedName(name)
}

// PrintFs prints the given filesystem to the given writer starting from the given path.
// This is useful for debugging.
func PrintFs(fs afero.Fs, path string, w io.Writer) {
	if fs == nil {
		return
	}

	afero.Walk(fs, path, func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
		return nil
	})
}
