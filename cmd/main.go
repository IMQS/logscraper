package main

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/IMQS/logscraper"
)

func main() {
	s := logscraper.NewScraper(getHostname(), "c:/imqsvar/logs/scraper-state.json", "c:/imqsvar/logs/scraper.log")
	s.LoadConfiguration("C:/imqsbin/static-conf/logscraper-config.json")

	// Comment out the following line when debugging
	s.SendToLoggly = true

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
