package logscraper

import (
	"regexp"
	"time"
)

const timeRFC8601_6Digits = "2006-01-02T15:04:05.999999Z0700"

var albionLogRegex *regexp.Regexp

func AlbionLogParser(msg []byte) *LogMsg {
	matches := albionLogRegex.FindSubmatchIndex(msg)
	if len(matches) != 10 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Time, err = time.Parse(timeRFC8601_6Digits, string(msg[matches[2]:matches[3]]))
	m.Severity = msg[matches[4]:matches[5]]
	m.ProcessID = msg[matches[6]:matches[7]]
	m.Message = msg[matches[8]:matches[9]]
	if err != nil {
		return nil
	}
	return m
}

func init() {
	// Example:
	// 2015-07-15T14:53:51.979201+0200 [I] 00001fdc Service: Starting
	albionLogRegex = regexp.MustCompile(`(\S+) \[(.)\] (\S+) (.*)`)
}
