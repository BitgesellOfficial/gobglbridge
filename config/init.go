package config

import (
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	yaml "gopkg.in/yaml.v2"
)

// reading config error is fatal, and exists main thread
func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

func readFile(cfg *Configuration) {
	f, err := os.Open("config.yml")
	if err != nil {
		processError(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		processError(err)
	}
}

func readEnv(cfg *Configuration) {
	err := envconfig.Process("", cfg)
	if err != nil {
		processError(err)
	}
}

func Init() {
	readFile(&Config)
	readEnv(&Config)
}
