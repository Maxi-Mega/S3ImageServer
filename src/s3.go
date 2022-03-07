package main

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"os"
	"path"
	"strings"
	"time"
)

func getFileFromBucket(minioClient *minio.Client, objKey, filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := minioClient.FGetObject(ctx, config.S3.BucketName, objKey, filePath, minio.GetObjectOptions{}); err != nil {
		handleS3Error(fmt.Errorf("failed to fetch file from s3 bucket => exit: %v", err))
		return fmt.Errorf("failed to fetch file from s3 bucket: %v", err)
	}
	return nil
}

func getImageFromBucket(minioClient *minio.Client, objKey, formattedKey, imgType string, lastModTime time.Time, eventChan chan event, updateOnly bool) error {
	filePath := path.Join(config.CacheDir, formattedKey)
	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}
	if eventChan != nil {
		if updateOnly {
			eventChan <- event{EventType: eventUpdate, EventObj: EventObject{ImgType: inferImageType(formattedKey).Name, ImgKey: formattedKey, ImgName: getGeoname(formattedKey)}, EventDate: lastModTime.String(), source: "getImageFromBucket"}
		} else {
			eventChan <- event{EventType: eventAdd, EventObj: EventObject{
				ImgType: imgType,
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
		// imagesCacheMutex.Lock()
		// delete(imagesCache, formattedKey)
		imagesCache.deleteImage(formattedKey)
		// imagesCacheMutex.Unlock()
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
		printError(fmt.Errorf("failed to delete file from cache: %v", err), false)
	}
	printDebug("Removed", fileName, "from cache")
}

func existsInCache(imgName string, obj minio.ObjectInfo) (exists, needsUpdate bool) {
	// if lastModTime, exist := imagesCache[imgName]; exist {
	if img, found := imagesCache.findImageByKey(imgName); found {
		lastModTime := img.LastModified
		if obj.LastModified.Before(lastModTime) || obj.LastModified.Equal(lastModTime) {
			return true, false
		}
		printDebug("Found updated image: ", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
		return true, true
	} else {
		printDebug("Found new image: ", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))
	}
	return false, false
}

func listMetaFiles(minioClient *minio.Client, dirs map[string]string, eventChan chan event) {
	printDebug(fmt.Sprintf("Looking for meta files in bucket [%s] ...", config.S3.BucketName))
	tempFullProductLinksCache := map[string][]string{}
	for dir, targetImg := range dirs {
		// logger.Info().Msg("Dir: " + dir)
		func() {
			tempFullProductLinksCache[dir] = []string{}
			ctx, cancel := context.WithTimeout(context.Background(), config.PollingPeriod)
			defer cancel()
			printDebug("Looking for meta files in ", dir, " | config.geonamesFilename: ", config.GeonamesFilename)
			for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: dir, Recursive: true}) {
				if obj.Err != nil {
					continue
				}

				if len(config.GeonamesFilename) > 0 && strings.HasSuffix(obj.Key, config.GeonamesFilename) {
					formattedFilename := formatFileName(dir + "/" + config.GeonamesFilename)
					if _, alreadyInCache := geonamesCache[formattedFilename]; alreadyInCache {
						continue
					}
					printDebug("Found geonames file: ", obj.Key)
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
		}()
	}
	fullProductLinksCacheMutex.Lock()
	fullProductLinksCache = tempFullProductLinksCache
	fullProductLinksCacheMutex.Unlock()
}

func extractFilesFromBucket(minioClient *minio.Client, eventChan chan event) error {
	pollMutex.Lock()
	defer pollMutex.Unlock()
	logger.Info().Msg(fmt.Sprintf("Looking for images in bucket [%s] ...", config.S3.BucketName))
	// previewBaseDirs := []string{}
	previewBaseDirs := map[string]string{}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, imgType := range config.ImageTypes {
		for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: imgType.Path, Recursive: true}) {
			if obj.Err != nil {
				handleS3Error(fmt.Errorf("no connection to S3 server => exit: %v", obj.Err))
				return obj.Err
			}

			if !strings.HasSuffix(obj.Key, config.PreviewFilename) {
				continue
			}

			if obj.LastModified.Add(config.RetentionPeriod).Before(time.Now()) {
				printDebug("Found image '", obj.Key, "', ignored because older than ", config.RetentionPeriod.String())
				continue
			}

			// previewBaseDirs = append(previewBaseDirs, obj.Key[:strings.LastIndex(obj.Key, "/")])
			previewBaseDirs[obj.Key[:strings.LastIndex(obj.Key, "/")]] = obj.Key

			formattedName := formatFileName(obj.Key)

			alreadyInCache, needsUpdate := existsInCache(obj.Key, obj)
			if alreadyInCache {
				img, found := imagesCache.findImageByKey(obj.Key)
				if found && img.LastModified.Equal(obj.LastModified) {
					continue
				}
			}

			err := getImageFromBucket(minioClient, obj.Key, formattedName, imgType.Name, obj.LastModified, eventChan, needsUpdate)
			if err != nil {
				return err
			}
			_, found := imagesCache.findImageByKey(obj.Key)
			if !found {
				imagesCache.addImage(obj.Key, obj.Size, obj.LastModified)
			}
			// imagesCacheMutex.Lock()
			// img.LastModified = obj.LastModified
			// imagesCacheMutex.Unlock()
		}
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
			// pollMutex.Lock()
			err := extractFilesFromBucket(minioClient, eventChan)
			if err != nil {
				printError(fmt.Errorf("failed to extract files from bucket: %v", err), false)
			}
			// pollMutex.Unlock()
		}
	}()
	printInfo("Started polling")
}

func listenToBucket(minioClient *minio.Client, eventChan chan event) {
	previewNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, "config.S3.KeyPrefix", config.PreviewFilename, []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"})
	geonamesNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, "config.S3.KeyPrefix", config.GeonamesFilename, []string{"s3:ObjectCreated:*"})
	fullProductNotifs := minioClient.ListenBucketNotification(context.Background(), config.S3.BucketName, "config.S3.KeyPrefix", config.FullProductExtension, []string{"s3:ObjectCreated:*"})

	go func() {
		printInfo("Starting to listen for bucket notifications ...")
		for {
			select {
			case notif := <-previewNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive preview notification: %v", err), false)
					continue
				}
				for _, e := range notif.Records {
					obj := e.S3.Object
					objKey := obj.Key
					formattedName := formatFileName(objKey)
					if strings.HasPrefix(e.EventName, "s3:ObjectCreated") {
						printDebug("[Created]: ", objKey)
						err := getImageFromBucket(minioClient, objKey, formattedName, inferImageType(objKey).Name, time.Now(), nil, false)
						if err != nil {
							printError(err, false)
							continue
						}
						// TODO: list full product images
						// imagesCacheMutex.Lock()
						// imagesCache[formattedName] = time.Now()
						imagesCache.addImage(objKey, obj.Size, time.Now())
						// imagesCacheMutex.Unlock()
						eventChan <- event{EventType: eventAdd, EventObj: EventObject{
							ImgType: inferImageType(formattedName).Name,
							ImgKey:  formattedName,
							ImgName: getGeoname(formattedName),
						}, EventDate: time.Now().String(),
							source: "listenToBucket"}
					} else if strings.HasPrefix(e.EventName, "s3:ObjectRemoved") {
						printDebug("[Removed]: ", objKey)
						deleteFileFromCache(formattedName)
						// imagesCacheMutex.Lock()
						// delete(imagesCache, formattedName)
						imagesCache.deleteImage(formattedName)
						// imagesCacheMutex.Unlock()
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
					printDebug("[Created geonames]: ", objKey)
					// formattedFilename := strings.ReplaceAll(objKey, "/", "@")
					// img, found := getCorrespondingImage(formattedFilename)
					img, found := imagesCache.findImageByKey(objKey)
					if !found {
						continue
					}
					err := getGeonamesFileFromBucket(minioClient, objKey, img.getAssociatedGeonamesPath(), img.FormattedKey, eventChan)
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
					printDebug("[Created full prod]: ", objKey)
					// formattedFilename := strings.ReplaceAll(objKey, "/", "@")
					// img, found := getCorrespondingImage(formattedFilename)
					img, found := imagesCache.findImageByKey(objKey)
					if !found {
						continue
					}
					imgDir := img.S3Key[:strings.LastIndex(img.S3Key, "/")]
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
