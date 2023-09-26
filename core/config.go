package logscraper

import (
	"errors"
	"fmt"

	serviceconfig "github.com/IMQS/serviceconfigsgo"
)

var parsersByName = map[string]Parser{
	"go":        GoLogParser,
	"spd":       SpdLogParser,
	"albion":    AlbionLogParser,
	"router":    RouterLogParser,
	"java":      JavaLogParser,
	"yellowfin": YellowfinLogParser,
}

const (
	serviceRegistryConfigFileName = "service-registry.json"
	serviceRegistryConfigVersion  = 1
	serviceName                   = "ImqsLogScraper"
)

type ServiceRegistryConfig struct {
	Services []struct {
		Logs []struct {
			Name     string `json:"name"`
			Filename string `json:"filename"`
			Parser   string `json:"parser"`
		} `json:"logs"`
	} `json:"services"`
}

func LoadServiceRegistryConfig(filename string) (*ServiceRegistryConfig, error) {
	cfg := &ServiceRegistryConfig{}

	err := serviceconfig.GetConfig(filename, serviceName, serviceRegistryConfigVersion, serviceRegistryConfigFileName, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Services == nil || len(cfg.Services) == 0 {
		return nil, errors.New("no services found in config file")
	}

	return cfg, err
}

func (config *ServiceRegistryConfig) LogSources() ([]*LogSource, []error) {
	logSources := make([]*LogSource, 0)
	errs := make([]error, 0)

	for _, v := range config.Services {
		for _, s := range v.Logs {
			if _, ok := parsersByName[s.Parser]; ok {
				logSources = append(logSources, NewLogSource(s.Name, s.Filename, parsersByName[s.Parser]))
			} else {
				errs = append(errs, fmt.Errorf("%s has parser %s which cannot be found", s.Name, s.Parser))
			}
		}
	}

	return logSources, errs
}
