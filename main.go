package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/IMQS/gowinsvc/service"
	logscraper "github.com/IMQS/logscraper/core"
)

func main() {

	ownhostname, _ := os.Hostname()
	s := logscraper.NewScraper(getHostname(), ownhostname, "c:/imqsvar/logs/scraper-state.json", "c:/imqsvar/logs/scraper.log")
	logscraper.InitialiseRelayers(s)

	conffile := flag.String("config", "", "Config file location")
	flag.Parse()

	err := s.LoadConfiguration(*conffile)
	if err != nil {
		log.Fatal(err)
	}

	// Comment out the following line when debugging
	s.SendToLoggly = true

	run := func() {
		s.Run()
	}
	if !service.RunAsService(run) {
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

	return ""
}
