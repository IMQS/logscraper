package logscraper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"errors"
)

var parsersByName = map[string]Parser{
	"go":        GoLogParser,
	"albion":    AlbionLogParser,
	"router":    RouterLogParser,
	"java":      JavaLogParser,
	"yellowfin": YellowfinLogParser,
}

type SourceConfig struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Parser   string `json:"parser"`
}

type LogScraperConfig struct {
	Sources []SourceConfig `json:sources`
}

func LoadLogScraperConfig(file string) (*LogScraperConfig, error) {
	if file == "" {
		return nil, errors.New("Config file cannot be nil")
	}

	var cfg LogScraperConfig
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, err
}

func (cfg *LogScraperConfig) validate() error {
	if cfg.Sources == nil || len(cfg.Sources) == 0 {
		return errors.New("No log sources found in config file")
	}
	return nil
}

func (config *LogScraperConfig) LogSources() ([]*LogSource, []error) {
	logSources := make([]*LogSource, len(config.Sources))
	errs := make([]error, 0)

	for ind, v := range config.Sources {
		if _, ok := parsersByName[v.Parser]; ok {
			logSources[ind] = NewLogSource(v.Name, v.Filename, parsersByName[v.Parser])
		} else {
			errs = append(errs, fmt.Errorf("%s has parser %s which cannot be found", v.Name, v.Parser))
		}
	}

	return logSources, errs
}
