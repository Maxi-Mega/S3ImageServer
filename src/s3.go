package main

import (
	"context"
	"fmt"
	"github.com/minio/minio-go"
	"os"
	"path"
	"strings"
	"time"
)

func getFileFromBucket(minioClient *minio.Client, objKey, formattedKey string, lastModTime time.Time, eventChan chan event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filePath := path.Join(config.CacheDir, formattedKey)
	err := minioClient.FGetObject(ctx, config.S3.BucketName, objKey, filePath, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	time.AfterFunc(config.RetentionPeriod, func() {
		delete(imagesCache, formattedKey)
		deleteImageFromCache(formattedKey)
		if !config.PollingMode && eventChan != nil {
			eventChan <- event{eventType: eventRemove, eventObj: formattedKey}
		}
	})
	return os.Chtimes(filePath, lastModTime, lastModTime)
}

func deleteImageFromCache(imgName string) {
	err := os.Remove(path.Join(config.CacheDir, imgName))
	if err != nil && !os.IsNotExist(err) {
		printError(err, false)
	}
	log("Removed", imgName, "from cache")
}

func existsInCache(imgName string, obj minio.ObjectInfo) bool {
	if lastModTime, exist := imagesCache[imgName]; exist {
		if obj.LastModified.Before(lastModTime) || obj.LastModified.Equal(lastModTime) {
			return true
		}
		log("Found updated image:", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
	} else {
		log("Found new image:", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
	}
	return false
}

func extractFilesFromBucket(minioClient *minio.Client, eventChan chan event) error {
	log(fmt.Sprintf("Looking for images in bucket [%s] ...", config.S3.BucketName))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: config.S3.KeyPrefix, Recursive: true}) {
		if obj.Err != nil {
			return obj.Err
		}

		if obj.LastModified.Add(config.RetentionPeriod).Before(time.Now()) {
			log("Found image '" + obj.Key + "', ignored because older than " + config.RetentionPeriod.String())
			continue
		}

		if !strings.HasSuffix(obj.Key, config.PreviewFilename) {
			continue
		}

		formattedName := formatImgName(obj.Key)

		alreadyInCache := existsInCache(formattedName, obj)
		if alreadyInCache {
			continue
		}

		err := getFileFromBucket(minioClient, obj.Key, formattedName, obj.LastModified, eventChan)
		if err != nil {
			return err
		}
		imagesCache[formattedName] = obj.LastModified
	}

	return nil
}

func pollBucket(minioClient *minio.Client) {
	go func() {
		for {
			time.Sleep(config.PollingPeriod)
			err := extractFilesFromBucket(minioClient, nil)
			if err != nil {
				printError(err, false)
			}
		}
	}()
}

func listenToBucket(minioClient *minio.Client, eventChan chan event) {
	events := []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"}
	notifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, config.S3.KeyPrefix, config.PreviewFilename, events)

	go func() {
		/*for {
			time.Sleep(5 * time.Second)
			eventChan <- event{eventType: eventAdd, eventObj: "preview.jpg"}
		}*/
		log("Starting to listen for bucket notifications ...")
		for {
			select {
			case notif := <-notifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive notification: %v", err), false)
					continue
				}
				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					formattedName := formatImgName(objKey)
					if strings.HasPrefix(e.EventName, "s3:ObjectCreated") {
						log("[Created]:", objKey)
						err := getFileFromBucket(minioClient, objKey, formattedName, time.Now(), eventChan)
						if err != nil {
							printError(err, false)
							continue
						}
						imagesCache[formattedName] = time.Now()
						eventChan <- event{eventType: eventAdd, eventObj: formattedName}
					} else if strings.HasPrefix(e.EventName, "s3:ObjectRemoved") {
						log("[Removed]:", objKey)
						deleteImageFromCache(formattedName)
						delete(imagesCache, formattedName)
						eventChan <- event{eventType: eventRemove, eventObj: formattedName}
					}
				}
			}
		}
	}()
}
