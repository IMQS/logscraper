package main

import (
	"github.com/IMQS/logscraper"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	s := logscraper.NewScraper(getHostname(), "c:/imqsvar/logs/scraper-state.json", "c:/imqsvar/logs/scraper.log")
	s.Sources = append(s.Sources, logscraper.NewLogSource("auth", "c:/imqsvar/logs/imqsauth.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("albion", "c:/imqsvar/logs/ImqsCpp.log", logscraper.AlbionLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("router_access", "c:/imqsvar/logs/router-access.log", logscraper.RouterLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("router_error", "c:/imqsvar/logs/router-error.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("search_access", "c:/imqsvar/logs/search-access.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("search_error", "c:/imqsvar/logs/search-error.log", logscraper.GoLogParser))
	run := func() {
		s.Run()
	}
	if !logscraper.RunAsService(run) {
		// run in foreground
		run()
	}
}

func getHostname() string {
	if hfile, err := ioutil.ReadFile("c:/imqsbin/conf/hostname"); err == nil && len(hfile) != 0 {
		line := string(hfile)
		if strings.Index(line, "http://") == 0 {
			return line[7:]
		} else if strings.Index(line, "https://") == 0 {
			return line[8:]
		} else {
			return line
		}
	}
	host, _ := os.Hostname()
	return host
}
