package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
)

const CONFIG_FILENAME = "config.toml"

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

type Config struct {
	CfgDir string

	data map[string]*Server
}

func NewConfig() (*Config, error) {
	home, ok := os.LookupEnv("HOME")
	if !ok || home == "" {
		return nil, fmt.Errorf("env var HOME not set")
	}

	configfp := fmt.Sprintf("%s/.config/reverse-proxy", home)
	err := os.MkdirAll(configfp, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("error making config dir: %v", err)
	}

	c := &Config{
		CfgDir: configfp,
		data:   map[string]*Server{},
	}

	c.readConfig()

	return c, nil
}

func (c *Config) readConfig() error {
	cfgFile := c.CfgDir + "/" + CONFIG_FILENAME
	ex, err := exists(cfgFile)
	if err != nil {
		return fmt.Errorf("error checking if file exists: %v", err)
	}
	if !ex {
		// is this an error or do we just default to no servers?
		c.data = map[string]*Server{}
		return nil
	}

	d, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	cfg := struct {
		Servers map[string]struct {
			Host          string `toml:"host"`
			DownstreamURL string `toml:"downstream-url"`
		} `toml:"servers"`
	}{}
	_, err = toml.Decode(string(d), &cfg)
	if err != nil {
		fmt.Println("error decoding toml:", err)
		return fmt.Errorf("error decoding toml: %v", err)
	}
	newServers := make(map[string]*Server)
	for _, server := range cfg.Servers {
		newServers[server.Host] = NewServer(server.Host, server.DownstreamURL)
	}

	c.data = newServers

	return nil
}

func (c *Config) GetServerForHost(host string) (*Server, bool) {
	s, ok := c.data[host]
	return s, ok
}

func (c *Config) Start() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("error making fs watcher", err)
		return
	}
	defer w.Close()

	err = w.Add(c.CfgDir)
	if err != nil {
		fmt.Println("error adding config path:", err)
		return
	}

	for {
		select {
		case e, ok := <-w.Events:
			if !ok {
				break
			}

			file := e.Name[len(c.CfgDir):]
			if file == "/config.toml" && e.Op.Has(fsnotify.Chmod) {
				// run the reload here
				fmt.Println("reloading config.toml")
				err := c.readConfig()
				if err != nil {
					fmt.Println("error reloading config:", err)
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				break
			}
			fmt.Println("[fs-error]", err)
		}
	}
}
