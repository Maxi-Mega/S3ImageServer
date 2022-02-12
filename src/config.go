package main

import (
	_ "embed"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
)

//go:embed resources/example_config.yml
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

	BasePath               string   `yaml:"basePath"`
	WindowTitle            string   `yaml:"windowTitle"`
	ScaleInitialPercentage uint8    `yaml:"scaleInitialPercentage"`
	PreviewFilename        string   `yaml:"previewFilename"`
	GeonamesFilename       string   `yaml:"geonamesFilename"`
	FullProductExtension   string   `yaml:"fullProductExtension"`
	FullProductProtocol    string   `yaml:"fullProductProtocol"`
	FullProductRootUrl     string   `yaml:"fullProductRootUrl"`
	FullProductSignedUrl   bool     `yaml:"fullProductSignedUrl"`
	ImageTypes             []string `yaml:"imageTypes"`

	LogLevel      string                 `yaml:"logLevel"`
	ColorLogs     bool                   `yaml:"colorLogs"`
	JsonLogFormat bool                   `yaml:"jsonLogFormat"`
	JsonLogFields map[string]interface{} `yaml:"jsonLogFields"`
	HttpTrace     bool                   `yaml:"httpTrace"`
	ExitOnS3Error bool                   `yaml:"exitOnS3Error"`

	CacheDir        string        `yaml:"cacheDir"`
	RetentionPeriod time.Duration `yaml:"retentionPeriod"`
	PollingMode     bool          `yaml:"pollingMode"`
	PollingPeriod   time.Duration `yaml:"pollingPeriod"`
	WebServerPort   uint16        `yaml:"webServerPort"`
}

var defaultConfig = Config{
	S3: S3Config{
		UseSSL: false,
	},

	BasePath:    "",
	WindowTitle: "S3 Image Viewer",

	LogLevel:      levelInfo,
	ColorLogs:     false,
	JsonLogFormat: false,
	JsonLogFields: map[string]interface{}{},
	HttpTrace:     false,
	ExitOnS3Error: false,
	CacheDir:      path.Join(os.TempDir(), defaultTempDirName),
	PollingMode:   false,
	PollingPeriod: 10 * time.Second,
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
		case "WindowTitle":
			if fieldValue.(string) == "" {
				config.WindowTitle = defaultConfig.WindowTitle
			}
		case "ScaleInitialPercentage":
			if fieldValue.(uint8) < 1 || fieldValue.(uint8) > 100 {
				config.ScaleInitialPercentage = 50
			}
		case "JsonLogFields":
			if fieldValue.(map[string]interface{}) == nil {
				config.JsonLogFields = defaultConfig.JsonLogFields
			}
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

	if len(config.LogLevel) == 0 {
		errs = append(errs, "no log level provided")
	} else {
		if config.LogLevel != levelDebug && config.LogLevel != levelInfo && config.LogLevel != levelWarn && config.LogLevel != levelError {
			errs = append(errs, "invalid log level")
		}
	}

	if config.RetentionPeriod == 0 {
		errs = append(errs, "no retention period provided")
	}
	if config.PollingMode && config.PollingPeriod == 0 {
		errs = append(errs, "no polling period provided")
	}

	return len(errs) == 0, errs
}

func loadConfigFromFile(filePath string) (Config, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("file %q not found", filePath)
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
				printWarn("Removed duplicate image type: ", imgType)
				cfg.ImageTypes = append(cfg.ImageTypes[:i], cfg.ImageTypes[i+1:]...)
			}
		}
	}
	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
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
	result += "basePath: " + config.BasePath + "\n"
	result += "windowTitle: " + config.WindowTitle + "\n"
	result += "scaleInitialPercentage: " + strconv.FormatUint(uint64(config.ScaleInitialPercentage), 10) + "\n"
	result += "previewFilename: " + config.PreviewFilename + "\n"
	result += "geonamesFilename: " + config.GeonamesFilename + "\n"
	result += "fullProductExtension: " + config.FullProductExtension + "\n"
	result += "fullProductProtocol: " + config.FullProductProtocol + "\n"
	result += "fullProductRootUrl: " + config.FullProductRootUrl + "\n"
	result += "fullProductSignedUrl: " + strconv.FormatBool(config.FullProductSignedUrl) + "\n"
	result += "imageTypes: " + strings.Join(config.ImageTypes, ", ") + "\n"
	result += fmt.Sprintf("logLevel: %s\ncolorLogs: %v\njsonLogFormat: %v\njsonLogFields: %v\nhttpTrace: %v\nexitOnS3Error: %v\n", config.LogLevel, config.ColorLogs, config.JsonLogFormat, config.JsonLogFields, config.HttpTrace, config.ExitOnS3Error)
	result += fmt.Sprintf("cacheDir: %s\npollingMode: %v\npollingPeriod: %v\nwebServerPort: %d\n", config.CacheDir, config.PollingMode, config.PollingPeriod, config.WebServerPort)
	return result
}

func printDefaultConfig() {
	fmt.Print("\nconfig.yml example:\n-------------------\n", defaultConfigFile, "\n\n")
}
