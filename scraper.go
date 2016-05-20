/* Package logscraper scrapes and parses the various IMQS log files

Some design considerations:
We don't keep log files open, because the Go standard library doesn't make it easy for us
to open files with SHARE_DELETE. Without this flag, we'd be preventing the log creators
from rolling their logs. So, when we detect that a log has been rolled, we try to find
the archived files, and make sure that we have read it all, before continuing onto the
new log file.

We assume that we will never encounter a half-written log entry. If that did happen,
then the scanner would read beyond that half-written entry, and we would record our
high-water mark as somewhere beyond that entry, thereby missing at least one log message.
*/
package logscraper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Parser func(msg []byte) *LogMsg

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type commonError int

const (
	commonErrorFileOpen      commonError = iota
	commonErrorSignatureSave             // this is common because the log file may have been rewound, but is still empty (or first line is too short)
)

type commonErrorLog map[commonError]uint64

// Increment the error count by one, and if the current value is a power of two, return true
func (l commonErrorLog) tick(err commonError) bool {
	l[err]++
	return (l[err]-1)&l[err] == 0
}

func (l commonErrorLog) reset(err commonError) {
	l[err] = 0
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type LogSource struct {
	Filename  string
	Name      string
	Parse     Parser
	firstLine []byte
	lastPos   int64
	errors    commonErrorLog
}

func NewLogSource(sourceName, filename string, parse Parser) *LogSource {
	s := &LogSource{
		Filename: filename,
		Name:     sourceName,
		Parse:    parse,
	}
	s.errors = make(commonErrorLog)
	return s
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// We separate LogMsg from logglyJsonMsg so that if we want to send our logs to a different format, it's straightforward.

type LogMsg struct {
	Time             time.Time
	Severity         []byte
	Message          []byte
	ProcessID        []byte
	ClientIP         []byte
	Request          []byte
	ResponseCode     []byte
	ResponseBytes    []byte
	ResponseDuration []byte
	JavaClass        []byte
}

type logglyJsonMsg struct {
	Host             string  `json:"host"`
	Source           string  `json:"source"`
	Time             string  `json:"timestamp"`
	Severity         string  `json:"severity,omitempty"`
	Message          string  `json:"message,omitempty"`
	ProcessID        int64   `json:"process_id,omitempty"`
	ClientIP         string  `json:"client_ip,omitempty"`
	Request          string  `json:"request,omitempty"`
	ResponseCode     string  `json:"response_code,omitempty"`
	ResponseBytes    int64   `json:"response_bytes,omitempty"`
	ResponseDuration float64 `json:"response_duration,omitempty"`
	JavaClass        string  `json:"java_class,omitempty"`
}

func (m *LogMsg) toLogglyJson(hostname string, source string, target *json.Encoder) error {
	pid, _ := strconv.ParseInt(string(m.ProcessID), 16, 64)
	respBytes, _ := strconv.ParseInt(string(m.ResponseBytes), 16, 64)
	respDuration, _ := strconv.ParseFloat(string(m.ResponseDuration), 64)
	j := logglyJsonMsg{
		Host:             hostname,
		Source:           source,
		Time:             m.Time.Format(timeRFC8601_6Digits),
		Severity:         string(m.Severity),
		Message:          string(m.Message),
		ProcessID:        pid,
		ClientIP:         string(m.ClientIP),
		Request:          string(m.Request),
		ResponseCode:     string(m.ResponseCode),
		ResponseBytes:    respBytes,
		ResponseDuration: respDuration,
		JavaClass:        string(m.JavaClass),
	}
	return target.Encode(&j)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type stateSourceJson struct {
	FirstLine []byte
	LastPos   int64
}

type stateJson struct {
	Sources map[string]stateSourceJson
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type Scraper struct {
	Sources       []*LogSource
	Hostname      string
	StateFilename string // Filename where we store our cached state (ie high-water mark of our log files)
	PollInterval  time.Duration
	SendToLoggly  bool
	metaLogFile   io.Writer
}

func NewScraper(hostname, statefile, metalogfile string) *Scraper {
	s := &Scraper{}
	if hostname != "" {
		s.Hostname = hostname
	} else {
		s.Hostname, _ = os.Hostname()
	}
	s.PollInterval = 30 * time.Second
	s.StateFilename = statefile
	if metalogfile != "" {
		s.metaLogFile = &lumberjack.Logger{
			Filename:   metalogfile,
			MaxSize:    30, // megabytes
			MaxBackups: 3,
		}
	} else {
		s.metaLogFile = os.Stdout
	}
	return s
}

func (s *Scraper) LoadConfiguration(file string) {
	config, err := LoadLogScraperConfig(file)
	if err != nil {
		fmt.Print(err)
		s.logMetaf("Error opening configuraton file: %v", err)
		return
	}

	logSources, errors := config.LogSources()
	if errors != nil && len(errors) > 0 {
		for _, err := range errors {
			s.logMetaf("Error parsing configuraton file: %v", err)
		}
		return
	}

	s.Sources = append(s.Sources, logSources...)
	for _, src := range s.Sources {
		fmt.Printf("Source loaded: %v\n", src)
	}
}

func (s *Scraper) Run() {
	s.logMetaf("Scraper starting")
	s.loadState()
	for {
		for _, src := range s.Sources {
			s.runSource(src)
		}
		s.saveState()
		time.Sleep(s.PollInterval)
	}
	s.logMetaf("Scraper exiting")
}

func (s *Scraper) runSource(src *LogSource) {
	raw, err := os.Open(src.Filename)
	if err != nil {
		if src.errors.tick(commonErrorFileOpen) {
			s.logMetaf("Error opening log file: %v", err)
		}
		return
	}
	src.errors.reset(commonErrorFileOpen)
	defer raw.Close()

	fileLength, err := raw.Seek(0, os.SEEK_END)
	if err != nil {
		s.logMetaf("Unable to seek to END on %v: %v", src.Filename, err)
		return
	}
	if fileLength < src.lastPos {
		s.logMetaf("Looks like a rewind on %v", src.Filename)
		// file has been rewound
		if err := s.handleLogRoll(src); err != nil {
			s.logMetaf("Log roll handling failed for %v: %v", src.Filename, err)
			return
		}
		if _, err := raw.Seek(0, os.SEEK_SET); err != nil {
			s.logMetaf("Unable to seek to 0 on %v: %v", src.Filename, err)
			return
		}
		s.logMetaf("%v has been rewound", src.Filename)
		src.lastPos = 0
		src.firstLine = nil
	}

	if src.lastPos == 0 {
		if err := s.saveFileSignature(raw, src); err != nil {
			if src.errors.tick(commonErrorSignatureSave) {
				// This can happen repeatedly, for a file that has been freshly created, but has too few
				// bytes in it for us to store a signature for it.
				s.logMetaf("Failed to save file signature of %v: %v", src.Filename, err)
			}
			return
		} else {
			s.logMetaf("Saved new signature of %v", src.Filename)
			src.errors.reset(commonErrorSignatureSave)
		}
	}

	if _, err = raw.Seek(src.lastPos, os.SEEK_SET); err != nil {
		s.logMetaf("Seek before scan failed: %v", err)
	}

	s.scan(raw, src)
}

func (s *Scraper) scan(logFile *os.File, src *LogSource) {
	scanner := bufio.NewScanner(logFile)

	output := &bytes.Buffer{}
	encoder := json.NewEncoder(output)

	// TODO: limit the number of lines that we scan in one go, to avoid sending a 100MB dump to loggly.
	// In order to do that, we'll probably have to use a lower-level scanning mechanism so that we can
	// get accurate seek positions when we stop.
	discarded := 0
	// Unparseable lines
	extraLines := []byte{}
	var prev_msg *LogMsg
	for scanner.Scan() {
		// We need to make a copy of scanner.Bytes(), because we store a message for one or more loop
		// iterations before dumping it to JSON. This does cause unnecessary GC pressure, but I'm leaving
		// it like this until the scraper becomes a performance hotspot.
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())
		msg := src.Parse(line)
		if msg != nil {
			if prev_msg != nil {
				prev_msg.Message = append(prev_msg.Message, extraLines...)
				prev_msg.toLogglyJson(s.Hostname, src.Name, encoder)
			} else {
				discarded += len(extraLines)
			}
			extraLines = []byte{}
			prev_msg = msg
		} else {
			// This might be multi-line message. Save it in a buffer, and append it to the previous message,
			// as soon as we find a new parseable message. By saving the lines in a buffer, we avoid storing
			// a half-written message from the end of the file.
			extraLines = append(extraLines, '\n')
			extraLines = append(extraLines, line...)
		}
	}
	if scanner.Err() != nil {
		s.logMetaf("Error reading log file %v: %v", src.Filename, scanner.Err())
		return
	}
	if prev_msg != nil {
		prev_msg.toLogglyJson(s.Hostname, src.Name, encoder)
	}
	if discarded != 0 {
		s.logMetaf("Discarded %v unparseable bytes from %v", discarded, src.Filename)
	}
	var err error
	if src.lastPos, err = logFile.Seek(0, os.SEEK_CUR); err != nil {
		s.logMetaf("Unable to find current file location after scanning %v: %v", src.Filename, err)
		return
	}
	fmt.Printf("Scanning %s, outputLen = %d\n", src.Filename, output.Len())
	if output.Len() != 0 {
		/*
			if dump, err := os.OpenFile("c:/imqsvar/logs/all.json", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err == nil {
				dump.Write(output.Bytes())
				dump.Close()
			} else {
				fmt.Printf("Cannot open dump file: %v\n", err)
			}*/
		//http.DefaultClient.Post("http://logs-01.loggly.com/bulk/9bc39e17-f062-4bef-9e28-b8456feaa999/tag/ImqsCpp", "application/json", bytes.NewReader(output.Bytes()))
		if s.SendToLoggly {
			var resp *http.Response
			resp, err := http.DefaultClient.Post("https://logs-01.loggly.com/bulk/9bc39e17-f062-4bef-9e28-b8456feaa999", "application/json", bytes.NewReader(output.Bytes()))
			if err != nil {
				s.logMetaf("Error posting log message to %v", err)
				return
			}
			resp.Body.Close()

		} else {
			fmt.Printf("Output:\n%v\n", string(output.Bytes()))
		}
	}
}

// This runs when we are seeing a fresh log file for the first time
func (s *Scraper) saveFileSignature(logFile *os.File, src *LogSource) error {
	sig, err := s.readFileSignature(logFile)
	if err != nil {
		return err
	}
	src.firstLine = sig
	_, err = logFile.Seek(0, os.SEEK_SET)
	return err
}

// Read the first 64 bytes of a file, so that we can recognize it after it has been renamed
func (s *Scraper) readFileSignature(file *os.File) ([]byte, error) {
	if _, err := file.Seek(0, os.SEEK_SET); err != nil {
		return nil, err
	}
	buf := [64]byte{}
	nread, err := file.Read(buf[:])
	if err != nil {
		return nil, err
	}
	if nread != 64 {
		return nil, io.EOF
	}
	return buf[:], nil
}

func (s *Scraper) handleLogRoll(src *LogSource) error {
	ext := path.Ext(src.Filename)
	wildcard := src.Filename[0:len(src.Filename)-len(ext)] + "*" + ext
	matches, err := filepath.Glob(wildcard)
	if err != nil {
		return err
	}
	var orgFile *os.File
	for _, match := range matches {
		if file, err := os.Open(match); err == nil {
			sig, _ := s.readFileSignature(file)
			if bytes.Equal(sig, src.firstLine) {
				s.logMetaf("Found matching archive of %v: %v", src.Filename, match)
				orgFile = file
				break
			}
			file.Close()
		}
	}

	if orgFile == nil {
		return nil
	}

	// Read the last few messages that were written into this log file
	// before it was archived.
	_, err = orgFile.Seek(src.lastPos, os.SEEK_SET)
	if err == nil {
		s.scan(orgFile, src)
	}
	orgFile.Close()
	return err
}

func (s *Scraper) loadState() {
	if s.StateFilename == "" {
		return
	}

	jraw, err := ioutil.ReadFile(s.StateFilename)
	if err != nil {
		s.logMetaf("Unable to read state file %v", err)
		return
	}

	jstate := stateJson{}
	if err = json.Unmarshal(jraw, &jstate); err != nil {
		s.logMetaf("Unable to parse state file %v: %v", s.StateFilename, err)
		return
	}

	for _, src := range s.Sources {
		if jstateItem, ok := jstate.Sources[src.Filename]; ok {
			src.firstLine = jstateItem.FirstLine
			src.lastPos = jstateItem.LastPos
		}
	}
}

func (s *Scraper) saveState() {
	if s.StateFilename == "" {
		return
	}

	jstate := stateJson{
		Sources: make(map[string]stateSourceJson),
	}
	for _, src := range s.Sources {
		jstate.Sources[src.Filename] = stateSourceJson{
			FirstLine: src.firstLine,
			LastPos:   src.lastPos,
		}
	}
	raw, err := json.MarshalIndent(&jstate, "", "\t")
	if err != nil {
		s.logMetaf("Error marshalling state: %v", err)
		return
	}

	err = ioutil.WriteFile(s.StateFilename, raw, 0666)
	if err != nil {
		s.logMetaf("Error writing state file: %v", err)
	}
}

func (s *Scraper) logMetaf(msg string, params ...interface{}) {
	str := time.Now().Format(timeRFC8601_6Digits) + " " + fmt.Sprintf(msg+"\n", params...)
	s.metaLogFile.Write([]byte(str))
}
