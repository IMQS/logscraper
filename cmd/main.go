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
	s.Sources = append(s.Sources, logscraper.NewLogSource("docs", "c:/imqsvar/logs/ImqsDocs.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("scheduler", "c:/imqsvar/logs/scheduler.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("ping", "c:/imqsvar/logs/service-ping.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("sap_webservice", "c:/imqsvar/logs/services/webservice.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("sap_operations", "c:/imqsvar/logs/services/operations.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("sap_notifications", "c:/imqsvar/logs/services/notifications.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("data_model_queries", "c:/imqsvar/logs/services/data-model-queries.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("pcs", "c:/imqsvar/logs/services/pcs/imqs-pcs-ws.log", logscraper.JavaLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("yellowfin", "c:/imqsvar/yellowfin/appserver/logs/yellowfin.log", logscraper.YellowfinLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("www_server", "c:/imqsvar/logs/www-server.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("www_js", "c:/imqsvar/logs/www-js.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("timeseries", "c:/imqsvar/logs/imqs-timeseries.log", logscraper.GoLogParser))
	s.Sources = append(s.Sources, logscraper.NewLogSource("insite", "c:/imqsvar/logs/insite.log", logscraper.GoLogParser))

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
