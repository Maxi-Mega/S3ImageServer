package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"os"
	"sync"
	"time"
)

const version = "1.8.0"

const defaultTempDirName = "s3_image_server"

var config Config

var imagesCache map[string]time.Time
var imagesCacheMutex sync.Mutex
var timers map[string]*time.Timer
var timersMutex sync.Mutex
var geonamesCache map[string]Geonames
var geonamesCacheMutex sync.Mutex
var fullProductLinksCache map[string][]string
var fullProductLinksCacheMutex sync.Mutex

var pollMutex sync.Mutex

// --config config.yml

func main() {
	var configPath string
	var printVersion bool
	var err error

	flag.Usage = func() {
		fmt.Println("S3 Image Server help:")
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
		fmt.Println("\nS3 Image Server | Version " + version)
		os.Exit(0)
	}

	if len(configPath) == 0 {
		exitWithError(errors.New("no configuration file provided (-c <file-path>)"))
	}

	config, err = loadConfigFromFile(configPath)
	if err != nil {
		exitWithError(fmt.Errorf("invalid configuration: %v", err))
	}

	if config.LogLevel == levelDebug {
		fmt.Println("\nStarting S3 Image Server " + version + " with configuration:")
		fmt.Println(config.String())
		fmt.Print("\n")
	} else {
		fmt.Println("\nStarting S3 Image Server " + version + " ...")
		fmt.Print("\n")
	}

	minioClient, err := minio.New(config.S3.EndPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.AccessId, config.S3.AccessSecret, ""),
		Secure: config.S3.UseSSL,
	})
	if err != nil {
		exitWithError(err)
	}

	initLogger()

	if config.HttpTrace {
		minioClient.TraceOn(os.Stdout)
	}

	printDebug("S3 endpoint:", minioClient.EndpointURL())

	if _, err = os.Stat(config.CacheDir); os.IsNotExist(err) {
		err = os.Mkdir(config.CacheDir, 0750)
		if err != nil {
			exitWithError(err)
		}
		imagesCache = map[string]time.Time{}
	} else {
		imagesCache = generateImagesCache()
	}

	eventChan := make(chan event, 1)
	timers = map[string]*time.Timer{}
	geonamesCache = map[string]Geonames{}

	go func() {
		if config.PollingMode {
			pollBucket(minioClient, eventChan)
			// err = startWebServer(config.WebServerPort)
			err = startWSServer(config.WebServerPort, eventChan)
		} else {
			listenToBucket(minioClient, eventChan)
			err = startWSServer(config.WebServerPort, eventChan)
		}
		if err != nil {
			exitWithError(err)
		}
	}()

	err = extractFilesFromBucket(minioClient, eventChan)
	if err != nil {
		exitWithError(fmt.Errorf("failed to extract files from bucket: %v", err))
	}

	printDebug("Found images have been stored in", config.CacheDir, "!")

	wg := new(sync.WaitGroup)
	wg.Add(1)
	wg.Wait()
}
