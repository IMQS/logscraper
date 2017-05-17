/*
LogReceivers are structs that define various log event endpoints that can
receive IMQSV8 log events. Any new receiver can be added by extending the
LogReceiver struct and implementing the Send(messages []*logMsg) interface method.

The initial design here was to decouple the main logscraper routine from the
actual sending of the events, as delays/issues in the sending to a receiver would
affect the main routine as well as timeous delivery to other receivers. However, seeing
that we only have 2 receivers at this stage, and no concrete requirements around
delivery time of events and retention, we'll be sending the events in the same routine.
This can always be changed by reintroducing the usage of the LogEvents channel ( or perhaps
sending the events to RabbitMQ), once we have firmer requirements.
*/

package logscraper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var receivers = make(map[string]Relay)
var datadogSeverities = map[string]bool{
	"ERROR": true,
	"E":     true,
	"FATAL": true,
	"F":     true,
}
var datadogSourceExclusions = map[string]bool{
	"www_js":    true,
	"yellowfin": true,
}

type Relay interface {
	Send(messages []*LogMsg)
	//Receive(messages []*LogMsg)
}

type LogReceiver struct {
	s         *Scraper
	URL       string
	ApiKey    string
	LogEvents chan []*LogMsg
}

type LogglyReceiver struct {
	LogReceiver
}

type DatadogReceiver struct {
	LogReceiver
	Host string
}

type logglyJsonMsg struct {
	Host             string  `json:"host"`
	OwnHostname      string  `json:"ownhostname"`
	Source           string  `json:"source"`
	Time             string  `json:"timestamp"`
	Severity         string  `json:"severity,omitempty"`
	Message          string  `json:"message,omitempty"`
	ProcessID        int64   `json:"process_id,omitempty"`
	ThreadID         int64   `json:"thread_id,omitempty"`
	ClientIP         string  `json:"client_ip,omitempty"`
	Request          string  `json:"request,omitempty"`
	ResponseCode     string  `json:"response_code,omitempty"`
	ResponseBytes    int64   `json:"response_bytes,omitempty"`
	ResponseDuration float64 `json:"response_duration,omitempty"`
	JavaClass        string  `json:"java_class,omitempty"`
}

type datadogJsonMessage struct {
	Host           string `json:"host"`
	Title          string `json:"title"`
	Text           string `json:"text"`
	Time           int64  `json:"date_happened"`
	Tags           string `json:"tags,omitempty"`
	AlertType      string `json:"alert_type"`
	AggregationKey string `json:"aggregation_key,omitempty"`
}

/*
Encodes all messages into a single json payload to send to Loggly
*/
func (lr *LogglyReceiver) Send(messages []*LogMsg) {
	output := &bytes.Buffer{}
	encoder := json.NewEncoder(output)
	for _, message := range messages {
		message.toLogglyJson(encoder)
	}

	resp, err := http.DefaultClient.Post(lr.URL+"/"+lr.ApiKey, "application/json", bytes.NewReader(output.Bytes()))
	if err != nil {
		lr.s.logMetaf("Error posting log message to %v", err)
		return
	}
	resp.Body.Close()
}

/*
Checks events for specific severities and sends them to Datadog individually
*/
func (dr *DatadogReceiver) Send(messages []*LogMsg) {
	//Datadog can't send an array of messages, we have to send them one-by-one.
	//This should be OK as we are only sending ERROR and FATAL messages.
	for _, message := range messages {
		_, severityOK := datadogSeverities[string(message.Severity)]
		_, sourceExcl := datadogSourceExclusions[string(message.Source)]

		if severityOK && !sourceExcl {
			output := &bytes.Buffer{}
			encoder := json.NewEncoder(output)
			message.toDatadogJson(dr.Host, encoder)

			resp, err := http.DefaultClient.Post(dr.URL+"?api_key="+dr.ApiKey, "application/json", bytes.NewReader(output.Bytes()))
			if err != nil {
				dr.s.logMetaf("Error posting log message to %v", err)
			}
			resp.Body.Close()
		}
	}
}

func (m *LogMsg) toDatadogJson(host string, target *json.Encoder) error {
	j := datadogJsonMessage{
		Host:           host,
		Title:          string(m.Source),
		Text:           string(m.Message),
		Time:           m.Time.Unix(),
		AlertType:      "error",
		AggregationKey: string(m.Source) + ":" + host,
	}
	return target.Encode(&j)
}

func (m *LogMsg) toLogglyJson(target *json.Encoder) error {
	pid, _ := strconv.ParseInt(string(m.ProcessID), 16, 64)
	tid, _ := strconv.ParseInt(string(m.ThreadID), 16, 64)
	respBytes, _ := strconv.ParseInt(string(m.ResponseBytes), 16, 64)
	respDuration, _ := strconv.ParseFloat(string(m.ResponseDuration), 64)
	j := logglyJsonMsg{
		Host:             string(m.Host),
		OwnHostname:      string(m.OwnHostname),
		Source:           string(m.Source),
		Time:             m.Time.Format(timeRFC8601_6Digits),
		Severity:         string(m.Severity),
		Message:          string(m.Message),
		ProcessID:        pid,
		ThreadID:         tid,
		ClientIP:         string(m.ClientIP),
		Request:          string(m.Request),
		ResponseCode:     string(m.ResponseCode),
		ResponseBytes:    respBytes,
		ResponseDuration: respDuration,
		JavaClass:        string(m.JavaClass),
	}
	return target.Encode(&j)
}

/*
Assigns specific configuration for the Datadog receiver based on
the installed agent's configuration. This includes the API key
and hostname.
*/
func (dr *DatadogReceiver) readDatadogCfg(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "api_key:") {
			dr.ApiKey = strings.TrimSpace(strings.Split(line, ":")[1])

		} else if strings.HasPrefix(line, "hostname:") {
			dr.Host = strings.TrimSpace(strings.Split(line, ":")[1])
		}
	}

	if dr.ApiKey == "" {
		return errors.New("No Datadog API key found")
	}
	//Use machine name if configured hostname is not specified.
	//This is how the Datadog agent behaves.
	if dr.Host == "" {
		dr.Host = dr.s.OwnHostname
	}

	return nil
}

/*
Initialises the receivers and assigns them to the global variable map.
*/
func InitialiseRelayers(s *Scraper) {

	//Loggly
	lgr := new(LogglyReceiver)
	lgr.s = s
	lgr.URL = "https://logs-01.loggly.com/bulk"
	lgr.ApiKey = "9bc39e17-f062-4bef-9e28-b8456feaa999"
	//lgr.LogEvents = make(chan []*LogMsg, 1000)
	//go lgr.Run(lgr)
	receivers["Loggly"] = lgr

	//Datadog, only if env set and DD conf found
	b, err := strconv.ParseBool(os.Getenv("IMQS_MONITOR"))
	if err == nil && b {
		dr := new(DatadogReceiver)
		err1 := dr.readDatadogCfg("C:\\ProgramData\\Datadog\\datadog.conf")
		if err1 == nil {
			dr.s = s
			dr.URL = "https://app.datadoghq.com/api/v1/events"
			//dr.LogEvents = make(chan []*LogMsg, 1000)
			//go dr.Run(dr)
			receivers["Datadog"] = dr

		} else {
			s.logMetaf("Datadog receiver not loaded. ", err1)
		}
	}

}

/*
Notifies all receivers of new messages to be sent
*/
func NotifyAllRelayers(messages []*LogMsg) {
	for _, value := range receivers {
		//value.Receive(messages)
		value.Send(messages)
	}
}

/*
func (lr *LogReceiver) Receive(messages []*LogMsg) {
	lr.LogEvents <- messages
}

func (lr *LogReceiver) Run(relay Relay) {
	lr.s.logMetaf("Starting Recevier: %v, apiKey:  %v", lr.URL, lr.ApiKey)
	for {
		select {
		case messages := <-lr.LogEvents:
			relay.Send(messages)
		}
	}
}*/
