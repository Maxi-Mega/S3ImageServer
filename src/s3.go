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

func getFileFromBucket(minioClient *minio.Client, objKey, filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return minioClient.FGetObject(ctx, config.S3.BucketName, objKey, filePath, minio.GetObjectOptions{})
}

func getImageFromBucket(minioClient *minio.Client, objKey, formattedKey string, lastModTime time.Time, eventChan chan event, updateOnly bool) error {
	filePath := path.Join(config.CacheDir, formattedKey)
	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		printError(err, false)
		return nil
	}
	if eventChan != nil {
		if updateOnly {
			eventChan <- event{EventType: eventUpdate, EventObj: EventObject{ImgType: getImageType(formattedKey), ImgKey: formattedKey, ImgName: getGeoname(formattedKey)}, EventDate: lastModTime.String(), source: "getImageFromBucket"}
		} else {
			eventChan <- event{EventType: eventAdd, EventObj: EventObject{
				ImgType: getImageType(formattedKey),
				ImgKey:  formattedKey,
				ImgName: getGeoname(formattedKey),
			}, EventDate: lastModTime.String(),
				source: "getImageFromBucket"}
		}
	}
	imageId := formattedKey // getImageId(formattedKey, lastModTime)
	timersMutex.Lock()
	if timer, found := timers[imageId]; found {
		timer.Stop()
	}
	timers[imageId] = time.AfterFunc(config.RetentionPeriod, func() {
		imagesCacheMutex.Lock()
		delete(imagesCache, formattedKey)
		imagesCacheMutex.Unlock()
		deleteFileFromCache(formattedKey)
		if eventChan != nil {
			eventChan <- event{EventType: eventRemove, EventObj: EventObject{ImgKey: formattedKey}}
		}
		delete(timers, imageId)
	})
	timersMutex.Unlock()
	return os.Chtimes(filePath, lastModTime, lastModTime)
}

func getGeonamesFileFromBucket(minioClient *minio.Client, objKey, formattedFilename, targetImg string, eventChan chan event) error {
	filePath := path.Join(config.CacheDir, formattedFilename)
	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}
	if timer, found := timers[formattedFilename]; found {
		timer.Stop()
	}
	geonames, err := parseGeonames(filePath)
	if err != nil {
		return err
	}
	geonamesCacheMutex.Lock()
	geonamesCache[formattedFilename] = geonames
	geonamesCacheMutex.Unlock()
	timersMutex.Lock()
	timers[formattedFilename] = time.AfterFunc(config.RetentionPeriod, func() {
		geonamesCacheMutex.Lock()
		delete(geonamesCache, formattedFilename)
		geonamesCacheMutex.Unlock()
		deleteFileFromCache(formattedFilename)
		timersMutex.Lock()
		delete(timers, formattedFilename)
		timersMutex.Unlock()
	})
	timersMutex.Unlock()
	eventChan <- event{
		EventType: eventGeonames,
		EventObj: EventGeonames{
			ImgKey:   targetImg,
			Geonames: getGeonamesTopLevel(geonames),
		},
		EventDate: time.Now().String(),
		source:    "getGeonamesFileFromBucket",
	}
	return nil
}

func deleteFileFromCache(fileName string) {
	err := os.Remove(path.Join(config.CacheDir, fileName))
	if err != nil && !os.IsNotExist(err) {
		printError(err, false)
	}
	log("Removed", fileName, "from cache")
}

func existsInCache(imgName string, obj minio.ObjectInfo) (exists, needsUpdate bool) {
	if lastModTime, exist := imagesCache[imgName]; exist {
		if obj.LastModified.Before(lastModTime) || obj.LastModified.Equal(lastModTime) {
			return true, false
		}
		log("Found updated image:", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
		return true, true
	} else {
		log("Found new image:", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
	}
	return false, false
}

func listMetaFiles(minioClient *minio.Client, dirs map[string]string, eventChan chan event) {
	log(fmt.Sprintf("Looking for meta files in bucket [%s] ...", config.S3.BucketName))
	tempFullProductLinksCache := map[string][]string{}
	for dir, targetImg := range dirs {
		tempFullProductLinksCache[dir] = []string{}
		ctx, cancel := context.WithTimeout(context.Background(), config.PollingPeriod)
		defer cancel()
		fmt.Println("=> Looking for meta files in", dir, "| config.geonamesFilename:", config.GeonamesFilename)
		for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: dir, Recursive: true}) {
			if obj.Err != nil {
				continue
			}

			if len(config.GeonamesFilename) > 0 && strings.HasSuffix(obj.Key, config.GeonamesFilename) {
				formattedFilename := formatFileName(dir + "/" + config.GeonamesFilename)
				if _, alreadyInCache := geonamesCache[formattedFilename]; alreadyInCache {
					continue
				}
				log("Found geonames file:", obj.Key)
				// targetImg := strings.ReplaceAll(dir, "/", "@") + config.PreviewFilename
				err := getGeonamesFileFromBucket(minioClient, obj.Key, formattedFilename, targetImg, eventChan)
				if err != nil {
					printError(err, false)
				}
				continue
			}

			if len(config.FullProductExtension) > 0 && strings.HasSuffix(obj.Key, config.FullProductExtension) {
				tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getFullProductImageLink(minioClient, obj.Key))
				continue
			}
		}
	}
	fullProductLinksCacheMutex.Lock()
	fullProductLinksCache = tempFullProductLinksCache
	fullProductLinksCacheMutex.Unlock()
}

func extractFilesFromBucket(minioClient *minio.Client, eventChan chan event) error {
	// log(fmt.Sprintf("Looking for images in bucket [%s] ...", config.S3.BucketName))
	// previewBaseDirs := []string{}
	previewBaseDirs := map[string]string{}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: config.S3.KeyPrefix, Recursive: true}) {
		if obj.Err != nil {
			return obj.Err
		}

		if !strings.HasSuffix(obj.Key, config.PreviewFilename) {
			continue
		}

		if obj.LastModified.Add(config.RetentionPeriod).Before(time.Now()) {
			log("Found image '" + obj.Key + "', ignored because older than " + config.RetentionPeriod.String())
			continue
		}

		// previewBaseDirs = append(previewBaseDirs, obj.Key[:strings.LastIndex(obj.Key, "/")])
		previewBaseDirs[obj.Key[:strings.LastIndex(obj.Key, "/")]] = obj.Key

		formattedName := formatFileName(obj.Key)

		alreadyInCache, needsUpdate := existsInCache(formattedName, obj)
		if alreadyInCache {
			if imagesCache[formattedName] == obj.LastModified {
				continue
			}
		}

		err := getImageFromBucket(minioClient, obj.Key, formattedName, obj.LastModified, eventChan, needsUpdate)
		if err != nil {
			return err
		}
		imagesCacheMutex.Lock()
		imagesCache[formattedName] = obj.LastModified
		imagesCacheMutex.Unlock()
	}

	listMetaFiles(minioClient, previewBaseDirs, eventChan)

	return nil
}

func pollBucket(minioClient *minio.Client, eventChan chan event) {
	go func() {
		startTime := time.Now()
		for {
			time.Sleep(config.PollingPeriod - time.Since(startTime))
			startTime = time.Now()
			pollMutex.Lock()
			err := extractFilesFromBucket(minioClient, eventChan)
			if err != nil {
				printError(err, false)
			}
			pollMutex.Unlock()
		}
	}()
	fmt.Println("Started polling")
}

func listenToBucket(minioClient *minio.Client, eventChan chan event) {
	previewNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, config.S3.KeyPrefix, config.PreviewFilename, []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"})
	geonamesNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, config.S3.KeyPrefix, config.GeonamesFilename, []string{"s3:ObjectCreated:*"})
	fullProductNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, config.S3.KeyPrefix, config.FullProductExtension, []string{"s3:ObjectCreated:*"})

	go func() {
		log("Starting to listen for bucket notifications ...")
		for {
			select {
			case notif := <-previewNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive preview notification: %v", err), false)
					continue
				}
				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					formattedName := formatFileName(objKey)
					if strings.HasPrefix(e.EventName, "s3:ObjectCreated") {
						log("[Created]:", objKey)
						err := getImageFromBucket(minioClient, objKey, formattedName, time.Now(), nil, false)
						if err != nil {
							printError(err, false)
							continue
						}
						// TODO: list full product images
						imagesCacheMutex.Lock()
						imagesCache[formattedName] = time.Now()
						imagesCacheMutex.Unlock()
						eventChan <- event{EventType: eventAdd, EventObj: EventObject{
							ImgType: getImageType(formattedName),
							ImgKey:  formattedName,
							ImgName: getGeoname(formattedName),
						}, EventDate: time.Now().String(),
							source: "listenToBucket"}
					} else if strings.HasPrefix(e.EventName, "s3:ObjectRemoved") {
						log("[Removed]:", objKey)
						deleteFileFromCache(formattedName)
						imagesCacheMutex.Lock()
						delete(imagesCache, formattedName)
						imagesCacheMutex.Unlock()
						eventChan <- event{EventType: eventRemove, EventObj: EventObject{ImgKey: formattedName}, source: "listenToBucket"}
					}
				}
			case notif := <-geonamesNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive geonames notification: %v", err), false)
					continue
				}
				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					log("[Created geonames]:", objKey)
					formattedFilename := strings.ReplaceAll(objKey, "/", "@")
					img, found := getCorrespondingImage(formattedFilename)
					if !found {
						continue
					}
					err := getGeonamesFileFromBucket(minioClient, objKey, img[:strings.LastIndex(img, "@")+1]+config.GeonamesFilename, img, eventChan)
					if err != nil {
						printError(err, false)
						continue
					}
				}
			case notif := <-fullProductNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive full product notification: %v", err), false)
					continue
				}
			RecordsLoop:
				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					log("[Created full prod]:", objKey)
					formattedFilename := strings.ReplaceAll(objKey, "/", "@")
					img, found := getCorrespondingImage(formattedFilename)
					if !found {
						continue
					}
					imgDir := strings.ReplaceAll(img[:strings.LastIndex(img, "@")], "@", "/")
					fullProductLink := getFullProductImageLink(minioClient, objKey)
					fullProductLinksCacheMutex.Lock()
					existingLinks, found := fullProductLinksCache[imgDir]
					if found {
						for _, link := range existingLinks {
							if link == fullProductLink {
								continue RecordsLoop
							}
						}
					} else {
						existingLinks = []string{}
					}
					fullProductLinksCache[imgDir] = append(existingLinks, fullProductLink)
					fullProductLinksCacheMutex.Unlock()
				}
			}
		}
	}()
}
