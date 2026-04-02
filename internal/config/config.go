package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type SSHConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	IdentityFile string `yaml:"identity_file"`
	InitCmd  string `yaml:"init_cmd,omitempty"`
}

type Project struct {
	Name           string `yaml:"name"`
	Path           string `yaml:"path"`
	Type           string `yaml:"type"` // "pm2" or "static"
	Branch         string `yaml:"branch"`
	PackageManager string `yaml:"package_manager"`
	InstallCmd     string `yaml:"install_cmd"`
	BuildCmd       string `yaml:"build_cmd"`
	PM2Name        string `yaml:"pm2_name,omitempty"`
	OutputDir      string `yaml:"output_dir,omitempty"`
}

type NginxConfig struct {
	ConfigPath string `yaml:"config_path"`
	SitesPath  string `yaml:"sites_path"`
}

type Config struct {
	AppName  string             `yaml:"app_name"`
	SSH      SSHConfig          `yaml:"ssh"`
	Projects map[string]Project `yaml:"projects"`
	Nginx    NginxConfig        `yaml:"nginx"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.SSH.IdentityFile = expandHomeDir(cfg.SSH.IdentityFile)

	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}

	return &cfg, nil
}

func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if runtime.GOOS == "windows" {
			return filepath.Join(home, path[2:])
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func (c *Config) GetProjectList() []string {
	keys := make([]string, 0, len(c.Projects))
	for k := range c.Projects {
		keys = append(keys, k)
	}
	return keys
}

func (c *Config) GetProject(key string) (Project, bool) {
	p, ok := c.Projects[key]
	return p, ok
}
