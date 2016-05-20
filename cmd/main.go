package main

import (
	"io/ioutil"
	"os"
	"strings"
	"log"
	"flag"
	"github.com/IMQS/logscraper"
)

func main() {
	s := logscraper.NewScraper(getHostname(), "c:/imqsvar/logs/scraper-state.json", "c:/imqsvar/logs/scraper.log")

	conffile := flag.String("config", "", "Config file location")
	flag.Parse()

	if *conffile == "" {
		log.Fatal("Usage: logscraper --config=/path/to/file.json")
	}

	err := s.LoadConfiguration(*conffile)
	if err != nil {
		log.Fatal(err)
	}

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
