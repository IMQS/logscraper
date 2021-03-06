package logscraper

import (
	"bytes"
	"regexp"
	"time"
)

const timeRFC8601_6Digits = "2006-01-02T15:04:05.000000Z0700"
const timeSpdLog = "2006-01-02T15:04:05.000000Z07:00"
const timeApache = "02/Jan/2006:15:04:05 -0700"
const timeJava = "2006-01-02 15:04:05.000 -0700"
const timeYellowfin = "2006-01-02 15:04:05"

var albionLogRegex *regexp.Regexp
var goLogRegex *regexp.Regexp
var spdLogRegex *regexp.Regexp
var javaLogRegex *regexp.Regexp
var routerLogRegex *regexp.Regexp
var yellowfinLogRegex *regexp.Regexp

func AlbionLogParser(msg []byte) *LogMsg {
	matches := albionLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (4+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Time, err = time.Parse(timeRFC8601_6Digits, string(getCapture(msg, matches, 0)))
	m.Severity = getCapture(msg, matches, 1)
	m.ProcessID = getCapture(msg, matches, 2)
	m.Message = getCapture(msg, matches, 3)
	if err != nil {
		return nil
	}
	return m
}

func SpdLogParser(msg []byte) *LogMsg {
	matches := spdLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (4+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Time, err = time.Parse(timeSpdLog, string(getCapture(msg, matches, 0)))
	m.Severity = getCapture(msg, matches, 1)
	m.ThreadID = getCapture(msg, matches, 2)
	m.Message = getCapture(msg, matches, 3)
	if err != nil {
		return nil
	}
	return m
}

func GoLogParser(msg []byte) *LogMsg {
	matches := goLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (3+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Time, err = time.Parse(timeRFC8601_6Digits, string(getCapture(msg, matches, 0)))
	m.Severity = getCapture(msg, matches, 1)
	m.Message = getCapture(msg, matches, 2)
	if err != nil {
		return nil
	}
	return m
}

func JavaLogParser(msg []byte) *LogMsg {
	matches := javaLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (6+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Severity = getCapture(msg, matches, 0)
	m.Time, err = time.Parse(timeJava, string(getCapture(msg, matches, 1)))
	// m.Thread = getCapture(msg, matches, 2)
	// m.MessageId = getCapture(msg, matches, 3)
	m.JavaClass = getCapture(msg, matches, 4)
	m.Message = getCapture(msg, matches, 5)
	if err != nil {
		return nil
	}
	return m
}

func RouterLogParser(msg []byte) *LogMsg {
	matches := routerLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (8+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.ClientIP = getCapture(msg, matches, 0)
	m.Time, err = time.Parse(timeApache, string(getCapture(msg, matches, 3)))
	m.Request = getCapture(msg, matches, 4)
	m.ResponseCode = getCapture(msg, matches, 5)
	m.ResponseBytes = getCapture(msg, matches, 6)
	m.ResponseDuration = getCapture(msg, matches, 7)
	if err != nil {
		return nil
	}
	return m
}

func YellowfinLogParser(msg []byte) *LogMsg {
	matches := yellowfinLogRegex.FindSubmatchIndex(msg)
	if len(matches) != (4+1)*2 {
		return nil
	}
	var err error
	m := &LogMsg{}
	m.Time, err = time.ParseInLocation(timeYellowfin, string(getCapture(msg, matches, 1)), time.Local)
	m.Severity = bytes.TrimSpace(getCapture(msg, matches, 2))
	m.Message = getCapture(msg, matches, 3)
	if err != nil {
		return nil
	}
	return m
}

// Extract a zero-based capture from a set of regex captures
// matches[0] .. matches[1] is the entire matched expression
// matches[2] .. matches[3] is first subexpression
// matches[4] .. matches[5] is second subexpression
// etc
func getCapture(msg []byte, matches []int, item int) []byte {
	item = (item + 1) * 2
	return msg[matches[item]:matches[item+1]]
}

func init() {
	// 2015-07-15T14:53:51.979201+0200 [I] 00001fdc Service: Starting
	albionLogRegex = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}\S+) \[([A-Z])\] ([0-9a-zA-Z]{8}) (.*)`)

	// 2015-07-15T14:53:51.979201+0200 [I] Service: Starting
	goLogRegex = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}\S+) \[([A-Z])\] (.*)`)

	// 2016-09-27T15:57:19.166825+02:00 [I] 22452 hello
	spdLogRegex = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}\S+) \[([A-Z])\] ([0-9]+) (.*)`)

	// INFO 2015-07-30 10:34:49.196 +0200 [pool-1-thread-1] org.eclipse.jetty.server.Server  jetty - 9.0.2.v20130417
	javaLogRegex = regexp.MustCompile(`(\S+)\s+(\d{4}-\d{2}-\d{2} \S+ \S+) (\S+)\s(\S*)\s\s(\S+)\s-\s(.*)`)

	// 127.0.0.1 - - [27/Jul/2015:15:15:45 +0200] "GET /albjs/tile_sc/... HTTP/1.1" 200 62223 3.8250
	routerLogRegex = regexp.MustCompile(`(\S+) (\S+) (\S+) \[([^\]]+)\] "([^"]+)" (\S+) (\S+) (\S+)`)

	// 2015-11-24 16:40:47: INFO (HtmlExporter:C) - Exporting report to HTML (56548: Existing Sewer Gravity Pipe Breakdown (by System Type))
	// 2015-12-02 02:00:00:ERROR (ReportRunner:M) - Error retrieving results: java.lang.Exception: Exception selecting data from database java.lang.Exception: Exception selecting data from database
	yellowfinLogRegex = regexp.MustCompile(`(\S+):(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}):(\s*\s*\S+)\s+(.*)`)
}
