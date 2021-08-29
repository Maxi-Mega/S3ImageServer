package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/credentials"
	"os"
	"time"
)

const version = "1.3.0"

const defaultTempDirName = "s3_image_server"

var config Config

var imagesCache map[string]time.Time

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

	if config.Debug {
		fmt.Println("Starting S3 Image Server " + version + " with configuration:")
		fmt.Println(config.String())
	} else {
		fmt.Println("Starting S3 Image Server " + version + " ...")
	}

	minioClient, err := minio.New(config.S3.EndPoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.AccessId, config.S3.AccessSecret, ""),
		Secure: config.S3.UseSSL,
	})
	if err != nil {
		exitWithError(err)
	}

	if config.HttpTrace {
		minioClient.TraceOn(os.Stdout)
	}

	log("S3 endpoint:", minioClient.EndpointURL())

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

	err = extractFilesFromBucket(minioClient, eventChan)
	if err != nil {
		exitWithError(fmt.Errorf("failed to extract files from bucket: %v", err))
	}

	log("Found images have been stored in", config.CacheDir, "!")

	if config.PollingMode {
		pollBucket(minioClient)
		err = startWebServer(config.WebServerPort)
	} else {
		listenToBucket(minioClient, eventChan)
		err = startWSServer(config.WebServerPort, eventChan)
	}
	if err != nil {
		exitWithError(err)
	}
}
