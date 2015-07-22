package main

import "github.com/IMQS/logscraper"

func main() {
	s := logscraper.NewScraper("c:/imqsvar/logs/scraper-state.json", "c:/imqsvar/logs/scraper.log")
	s.Sources = append(s.Sources, logscraper.NewLogSource("c:/imqsvar/logs/ImqsCpp.log", logscraper.AlbionLogParser))
	s.Run()
}
