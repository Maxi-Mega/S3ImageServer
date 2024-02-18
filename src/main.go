package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var version = "3.3.2-dev"

const defaultTempDirName = "s3_image_server"

var config Config //nolint:gochecknoglobals

//nolint:gochecknoglobals
var (
	mainCache                        ImageCache
	thumbnailsCache                  ImageCache
	imagesCacheMutex                 sync.Mutex
	timers                           map[string]*time.Timer
	timersMutex                      sync.Mutex
	geonamesCache                    map[string]Geonames
	geonamesCacheMutex               sync.Mutex
	localizationCache                map[string]Localization
	localizationCacheMutex           sync.Mutex
	featuresCache                    map[string]Features
	featuresCacheMutex               sync.Mutex
	fullProductLinksCache            map[string][]string //nolint: godox // TODO: rename ?
	fullProductLinksCacheMutex       sync.Mutex
	additionalProductFilesCache      map[string]time.Time
	additionalProductFilesCacheMutex sync.Mutex
)

var pollMutex sync.Mutex //nolint:gochecknoglobals

func main() {
	var (
		configPath   string
		printVersion bool
		err          error
	)

	flag.Usage = func() {
		log.Println("S3 Image Server help:")
		flag.PrintDefaults()
		printDefaultConfig()
	}

	flag.StringVar(&configPath, "c", "", "config file path")
	flag.BoolVar(&printVersion, "v", false, "software version")
	flag.Parse()

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	if printVersion {
		log.Println("\nS3 Image Server | Version " + version)
		os.Exit(0)
	}

	if len(configPath) == 0 {
		exitWithError(errors.New("no configuration file provided (-c <file-path>)"))
	}

	config, err = loadConfigFromFile(configPath)
	if err != nil {
		exitWithError(fmt.Errorf("invalid configuration: %w", err))
	}

	if config.LogLevel == levelDebug {
		log.Println("\nStarting S3 Image Server " + version + " with configuration:")
		log.Println(config.String() + "\n")
	} else {
		log.Println("\nStarting S3 Image Server " + version + " ...\n")
	}

	minioClient, err := minio.New(config.S3.EndPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.AccessID, config.S3.AccessSecret, ""),
		Secure: config.S3.UseSSL,
	})
	if err != nil {
		exitWithError(err)
	}

	initLogger()

	if config.HTTPTrace {
		minioClient.TraceOn(os.Stdout)
	}

	printDebug("S3 endpoint:", minioClient.EndpointURL())

	mainCache = createCache(config.mainCacheDir)
	thumbnailsCache = createCache(config.thumbnailsCacheDir)

	eventChan := make(chan event, 1)
	timers = make(map[string]*time.Timer)
	geonamesCache = make(map[string]Geonames)
	localizationCache = make(map[string]Localization)
	featuresCache = make(map[string]Features)
	additionalProductFilesCache = make(map[string]time.Time)

	go func() {
		if config.PollingMode {
			pollBucket(minioClient, eventChan)
		} else {
			listenToBucket(minioClient, eventChan)
		}

		err = startWSServer(config.WebServerPort, eventChan, minioClient)
		if err != nil {
			exitWithError(err)
		}
	}()

	err = extractFilesFromBucket(minioClient, eventChan)
	if err != nil {
		exitWithError(fmt.Errorf("failed to extract files from bucket: %w", err))
	}

	printDebug("S3 images have been stored in ", config.mainCacheDir)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()
}
