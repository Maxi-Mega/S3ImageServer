package main

import (
	_ "embed"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"path"
	"reflect"
	"strings"
	"time"
)

//go:embed example_config.yml
var defaultConfigFile string

// S3Config is the directly related part of the global config
type S3Config struct {
	EndPoint     string `yaml:"endPoint"`
	BucketName   string `yaml:"bucketName"`
	KeyPrefix    string `yaml:"keyPrefix"`
	AccessId     string `yaml:"accessId"`
	AccessSecret string `yaml:"accessSecret"`
	UseSSL       bool   `yaml:"useSSL"`
}

type Config struct {
	S3 S3Config `yaml:"s3"`

	PreviewFilename string   `yaml:"previewFilename"`
	ImageTypes      []string `yaml:"imageTypes"`

	Debug           bool          `yaml:"debug"`
	HttpTrace       bool          `yaml:"httpTrace"`
	CacheDir        string        `yaml:"cacheDir"`
	RetentionPeriod time.Duration `yaml:"retentionPeriod"`
	PollingMode     bool          `yaml:"pollingMode"`
	PollingPeriod time.Duration `yaml:"pollingPeriod"`
	WebServerPort   uint16        `yaml:"webServerPort"`
}

var defaultConfig = Config{
	S3: S3Config{
		UseSSL: false,
	},

	Debug:         false,
	HttpTrace:     false,
	CacheDir:      path.Join(os.TempDir(), defaultTempDirName),
	PollingMode:   false,
	WebServerPort: 9999,
}

func (config *Config) loadDefaults() {
	v := reflect.ValueOf(*config)
	for f := 0; f < v.NumField(); f++ {
		field := v.Field(f)
		fieldName := v.Type().Field(f).Name
		fieldValue := field.Interface()
		switch fieldName {
		case "S3":
			/*s3Config := fieldValue.(S3Config)
			if s3Config.UseSSL == false {
				config.S3.UseSSL = defaultConfig.S3.UseSSL
			}*/
		case "Debug":
			/*if fieldValue.(bool) == false {
				config.Debug = defaultConfig.Debug
			}*/
		case "CacheDir":
			if fieldValue.(string) == "" {
				config.CacheDir = defaultConfig.CacheDir
			}
		case "WebServerPort":
			if fieldValue.(uint16) == 0 {
				config.WebServerPort = defaultConfig.WebServerPort
			}
		}
	}
}

func (config *Config) checkValidity() (bool, []string) {
	errs := []string{}
	if config.S3.EndPoint == "" {
		errs = append(errs, "no s3 endpoint provided")
	}
	if config.S3.BucketName == "" {
		errs = append(errs, "no s3 bucket name provided")
	}
	if config.S3.AccessId == "" {
		errs = append(errs, "no s3 access id provided")
	}
	if config.S3.AccessSecret == "" {
		errs = append(errs, "no s3 access secret provided")
	}

	if len(config.ImageTypes) == 0 {
		errs = append(errs, "no image type provided")
	}

	if config.RetentionPeriod == 0 {
		errs = append(errs, "no retention period provided")
	}
	if config.PollingMode && config.PollingPeriod == 0 {
		errs = append(errs, "no polling period provided")
	}

	return len(errs) == 0, errs
}

func loadConfigFromFile(filepath string) (Config, error) {
	fileContent, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("file %q not found", filepath)
		}
		return Config{}, err
	}
	var cfg Config
	err = yaml.Unmarshal(fileContent, &cfg)
	if err != nil {
		return Config{}, err
	}
	for i, imgType := range cfg.ImageTypes {
		for j, iT := range cfg.ImageTypes {
			if i != j && imgType == iT {
				log("Removed duplicate image type:", imgType)
				cfg.ImageTypes = append(cfg.ImageTypes[:i], cfg.ImageTypes[i+1:]...)
			}
		}
	}
	cfg.loadDefaults()
	valid, errs := cfg.checkValidity()
	if !valid {
		return Config{}, errors.New(strings.Join(errs, ", "))
	}
	return cfg, nil
}

func (config Config) String() string {
	result := "S3:\n"
	s3 := config.S3
	result += fmt.Sprintf("\tendPoint: %s\n\tbucketName: %s\n\tkeyPrefix: %s\n\taccessId: %s\n\taccessSecret: %s\n", s3.EndPoint, s3.BucketName, s3.KeyPrefix, s3.AccessId, s3.AccessSecret)
	result += "previewFilename: " + config.PreviewFilename + "\n"
	result += "imageTypes: " + strings.Join(config.ImageTypes, ", ") + "\n"
	result += fmt.Sprintf("debug: %v\ncacheDir: %s\nwebServerPort: %d\n", config.Debug, config.CacheDir, config.WebServerPort)
	return result
}

func printDefaultConfig() {
	fmt.Print("\nconfig.yml example:\n-------------------\n", defaultConfigFile, "\n\n\n")
}
