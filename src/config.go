package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	mainCacheDirName       = "main"
	thumbnailsCacheDirName = "thumbnails"
)

//go:embed resources/example_config.yml
var defaultConfigFile string

// S3Config is the directly related part of the global config.
type S3Config struct {
	EndPoint   string `yaml:"endPoint"`
	BucketName string `yaml:"bucketName"`
	// KeyPrefix    string `yaml:"keyPrefix"`
	AccessID     string `yaml:"accessId"`
	AccessSecret string `yaml:"accessSecret"`
	UseSSL       bool   `yaml:"useSSL"`
}

type ImageType struct {
	Name          string `json:"name"		   yaml:"name"`
	DisplayName   string `json:"displayName"   yaml:"displayName"`
	ProductPrefix string `json:"productPrefix" yaml:"productPrefix"`
	ProductRegexp string `json:"productRegexp" yaml:"productRegexp"`
	productRegexp *regexp.Regexp
}

type ImageGroup struct {
	GroupName string      `yaml:"groupName"`
	Types     []ImageType `yaml:"types"`
}

type Config struct {
	S3 S3Config `yaml:"s3"`

	BasePath                     string `yaml:"basePath"`
	WindowTitle                  string `yaml:"windowTitle"`
	ScaleInitialPercentage       uint8  `yaml:"scaleInitialPercentage"`
	PreviewFilename              string `yaml:"previewFilename"`
	GeonamesFilename             string `yaml:"geonamesFilename"`
	LocalizationFilename         string `yaml:"localizationFilename"`
	AdditionalProductFilesRegexp string `yaml:"additionalProductFilesRegexp"`
	additionalProductFilesRegexp *regexp.Regexp
	TileServerURL                string `yaml:"tileServerURL"`
	FeaturesExtensionRegexp      string `yaml:"featuresExtensionRegexp"`
	featuresExtensionRegexp      *regexp.Regexp
	FeaturesCategoryName         string       `yaml:"featuresCategoryName"`
	FeaturesClassName            string       `yaml:"featuresClassName"`
	FullProductExtension         string       `yaml:"fullProductExtension"`
	FullProductProtocol          string       `yaml:"fullProductProtocol"`
	FullProductRootURL           string       `yaml:"fullProductRootUrl"`
	FullProductSignedURL         bool         `yaml:"fullProductSignedUrl"`
	ImageGroups                  []ImageGroup `yaml:"imageGroups"`
	imageTypes                   []ImageType

	LogLevel      string                 `yaml:"logLevel"`
	ColorLogs     bool                   `yaml:"colorLogs"`
	JSONLogFormat bool                   `yaml:"jsonLogFormat"`
	JSONLogFields map[string]interface{} `yaml:"jsonLogFields"`
	HTTPTrace     bool                   `yaml:"httpTrace"`
	ExitOnS3Error bool                   `yaml:"exitOnS3Error"`

	BaseCacheDir          string `yaml:"cacheDir"`
	mainCacheDir          string
	thumbnailsCacheDir    string
	RetentionPeriod       time.Duration `yaml:"retentionPeriod"`
	MaxImagesDisplayCount int           `yaml:"maxImagesDisplayCount"`
	PollingMode           bool          `yaml:"pollingMode"`
	PollingPeriod         time.Duration `yaml:"pollingPeriod"`
	WebServerPort         uint16        `yaml:"webServerPort"`
}

var defaultConfig = Config{ //nolint:gochecknoglobals
	S3: S3Config{
		UseSSL: false,
	},

	BasePath:    "",
	WindowTitle: "S3 Image Viewer",

	LogLevel:      levelInfo,
	ColorLogs:     false,
	JSONLogFormat: false,
	JSONLogFields: map[string]interface{}{},
	HTTPTrace:     false,
	ExitOnS3Error: false,
	BaseCacheDir:  filepath.Join(os.TempDir(), defaultTempDirName),
	PollingMode:   false,
	PollingPeriod: 10 * time.Second,
	WebServerPort: 9999,
}

func (config *Config) loadDefaults() {
	v := reflect.ValueOf(*config)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)

		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}

		fieldName := field.Name
		fieldValue := f.Interface()

		switch fieldName {
		case "S3":
		/*s3Config := fieldValue.(S3Config)
		if s3Config.UseSSL == false {
			config.S3.UseSSL = defaultConfig.S3.UseSSL
		}*/
		case "WindowTitle":
			if fieldValue.(string) == "" { //nolint: forcetypeassert
				config.WindowTitle = defaultConfig.WindowTitle
			}
		case "ScaleInitialPercentage":
			if fieldValue.(uint8) < 1 || fieldValue.(uint8) > 100 { //nolint: forcetypeassert
				config.ScaleInitialPercentage = 50
			}
		case "JsonLogFields":
			if fieldValue.(map[string]interface{}) == nil { //nolint: forcetypeassert
				config.JSONLogFields = defaultConfig.JSONLogFields
			}
		case "BaseCacheDir":
			if fieldValue.(string) == "" { //nolint: forcetypeassert
				config.BaseCacheDir = defaultConfig.BaseCacheDir
			}
		case "WebServerPort":
			if fieldValue.(uint16) == 0 { //nolint: forcetypeassert
				config.WebServerPort = defaultConfig.WebServerPort
			}
		}
	}
}

func (config *Config) checkValidity() (ok bool, errs []string) {
	if config.S3.EndPoint == "" {
		errs = append(errs, "no s3 endpoint provided")
	}

	if config.S3.BucketName == "" {
		errs = append(errs, "no s3 bucket name provided")
	}

	if config.S3.AccessID == "" {
		errs = append(errs, "no s3 access id provided")
	}

	if config.S3.AccessSecret == "" {
		errs = append(errs, "no s3 access secret provided")
	}

	if len(config.ImageGroups) == 0 {
		errs = append(errs, "no image group provided")
	}

	imageTypes := make(map[string]struct{})
	imagePaths := make(map[string]struct{})

	for i, group := range config.ImageGroups {
		if group.GroupName == "" {
			errs = append(errs, "no name provided for group n°"+strconv.Itoa(i))
			continue
		}

		if len(group.Types) == 0 {
			errs = append(errs, "no image type provided in group "+group.GroupName)
			continue
		}

		for _, imageType := range group.Types {
			if _, exists := imageTypes[imageType.Name]; exists {
				errs = append(errs, "image type '"+imageType.Name+"' is present in multiple groups")
			} else {
				imageTypes[imageType.Name] = struct{}{}
			}

			if _, exists := imagePaths[imageType.ProductPrefix]; exists {
				errs = append(errs, "image path '"+imageType.ProductPrefix+"' is present in multiple groups")
			} else {
				imagePaths[imageType.ProductPrefix] = struct{}{}
			}
		}
	}

	if len(config.LogLevel) == 0 {
		errs = append(errs, "no log level provided")
	} else { //nolint: gocritic
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

		return Config{}, err //nolint:wrapcheck
	}

	var cfg Config

	err = yaml.Unmarshal(fileContent, &cfg)
	if err != nil {
		return Config{}, err //nolint:wrapcheck
	}

	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
	cfg.loadDefaults()

	valid, errs := cfg.checkValidity()
	if !valid {
		return Config{}, errors.New(strings.Join(errs, "\n- "))
	}

	if cfg.AdditionalProductFilesRegexp != "" {
		cfg.additionalProductFilesRegexp, err = regexp.Compile(cfg.AdditionalProductFilesRegexp)
		if err != nil {
			return Config{}, fmt.Errorf("invalid additional product files regexp: %w", err)
		}
	}

	if cfg.FeaturesExtensionRegexp != "" {
		cfg.featuresExtensionRegexp, err = regexp.Compile(cfg.FeaturesExtensionRegexp)
		if err != nil {
			return Config{}, fmt.Errorf("invalid features extension regexp: %w", err)
		}
	}

	for _, group := range cfg.ImageGroups {
		for i := range group.Types {
			imgType := group.Types[i]
			if imgType.ProductRegexp == "" {
				return Config{}, fmt.Errorf("no product regexp provided for type %q of group %q", imgType.Name, group.GroupName)
			}

			imgType.productRegexp, err = regexp.Compile(imgType.ProductRegexp)
			if err != nil {
				return Config{}, fmt.Errorf("invalid product regexp for type %q of group %q: %w", imgType.Name, group.GroupName, err)
			}

			cfg.imageTypes = append(cfg.imageTypes, imgType)
		}
	}

	cfg.mainCacheDir = filepath.Join(cfg.BaseCacheDir, mainCacheDirName)
	cfg.thumbnailsCacheDir = filepath.Join(cfg.BaseCacheDir, thumbnailsCacheDirName)

	return cfg, nil
}

func (config *Config) String() string {
	result := "S3:\n"
	s3 := config.S3
	result += fmt.Sprintf("\tendPoint: %s\n\tbucketName: %s\n\taccessId: %s\n\taccessSecret: %s\n", s3.EndPoint, s3.BucketName, s3.AccessID, s3.AccessSecret)
	result += "basePath: " + config.BasePath + "\n"
	result += "windowTitle: " + config.WindowTitle + "\n"
	result += "scaleInitialPercentage: " + strconv.FormatUint(uint64(config.ScaleInitialPercentage), 10) + "\n"
	result += "previewFilename: " + config.PreviewFilename + "\n"
	result += "geonamesFilename: " + config.GeonamesFilename + "\n"
	result += "additionalProductFilesRegexp: " + config.AdditionalProductFilesRegexp + "\n"
	result += "tileServerURL: " + config.TileServerURL + "\n"
	result += "featuresExtensionRegexp: " + config.FeaturesExtensionRegexp + "\n"
	result += "featuresCategoryName: " + config.FeaturesCategoryName + "\n"
	result += "featuresClassName: " + config.FeaturesClassName + "\n"
	result += "fullProductExtension: " + config.FullProductExtension + "\n"
	result += "fullProductProtocol: " + config.FullProductProtocol + "\n"
	result += "fullProductRootUrl: " + config.FullProductRootURL + "\n"
	result += "fullProductSignedUrl: " + strconv.FormatBool(config.FullProductSignedURL) + "\n"
	result += "imageGroups: " + joinStructs(config.ImageGroups, ", ", false) + "\n"
	result += fmt.Sprintf("logLevel: %s\ncolorLogs: %v\njsonLogFormat: %v\njsonLogFields: %v\nhttpTrace: %v\nexitOnS3Error: %v\n", config.LogLevel, config.ColorLogs, config.JSONLogFormat, config.JSONLogFields, config.HTTPTrace, config.ExitOnS3Error)
	result += fmt.Sprintf("cacheDir: %s\nretentionPeriod: %v\nmaxImagesDisplayCount: %d\npollingMode: %v\npollingPeriod: %v\nwebServerPort: %d\n", config.mainCacheDir, config.RetentionPeriod, config.MaxImagesDisplayCount, config.PollingMode, config.PollingPeriod, config.WebServerPort)

	return result
}

func printDefaultConfig() {
	fmt.Print("\nconfig.yml example:\n-------------------\n", defaultConfigFile, "\n\n") //nolint: forbidigo
}
