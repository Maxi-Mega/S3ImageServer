package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

func getFileFromBucket(minioClient *minio.Client, objKey, filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := minioClient.FGetObject(ctx, config.S3.BucketName, objKey, filePath, minio.GetObjectOptions{}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			printWarn(fmt.Sprintf("Context deadline exceeded while getting object %q", objKey))
		}

		handleS3Error(fmt.Errorf("failed to fetch file from s3 bucket => exit: %w", err))

		return fmt.Errorf("failed to fetch file from s3 bucket: %w", err)
	}

	return nil
}

func getImageFromBucket(cache ImageCache, minioClient *minio.Client, objKey, formattedKey, imgType string, lastModTime time.Time, eventChan chan event, updateOnly bool) error {
	filePath := filepath.Join(cache.pathOnDisk, formattedKey)

	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}

	if eventChan != nil {
		if updateOnly {
			eventChan <- event{EventType: eventUpdate, EventObj: EventObject{
				ImgType: inferImageType(formattedKey).Name,
				ImgKey:  formattedKey,
				ImgName: getGeoname(formattedKey),
				ImgDate: lastModTime.In(time.Local).Format("2006-01-02 15:04:05 MST"), //nolint:gosmopolitan
			}, EventDate: lastModTime.String(), source: "getImageFromBucket"}
		} else {
			eventChan <- event{EventType: eventAdd, EventObj: EventObject{
				ImgType: imgType,
				ImgKey:  formattedKey,
				ImgName: getGeoname(formattedKey),
				ImgDate: lastModTime.In(time.Local).Format("2006-01-02 15:04:05 MST"), //nolint:gosmopolitan
			}, EventDate: lastModTime.String(),
				source: "getImageFromBucket"}
		}
	}

	imageID := formattedKey

	timersMutex.Lock()

	if timer, found := timers[imageID]; found {
		timer.Stop()
	}

	timers[imageID] = time.AfterFunc(config.RetentionPeriod, func() {
		cache.deleteImage(formattedKey)
		deleteFileFromCache(formattedKey)

		if eventChan != nil {
			eventChan <- event{EventType: eventRemove, EventObj: EventObject{ImgKey: formattedKey}}
		}

		delete(timers, imageID)
	})

	timersMutex.Unlock()

	return os.Chtimes(filePath, lastModTime, lastModTime) //nolint:wrapcheck
}

func getGeonamesFileFromBucket(minioClient *minio.Client, objKey string, objDate time.Time, formattedFilename, targetImg string, eventChan chan event) error {
	filePath := filepath.Join(config.mainCacheDir, formattedFilename)

	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}

	if timer, found := timers[formattedFilename]; found {
		timer.Stop()
	}

	geonames, err := parseGeonames(filePath, objDate)
	if err != nil {
		return err
	}

	img, found := mainCache.findImageByPrefix(targetImg)
	if found {
		img.AssociatedGeonames = &geonames
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
			Geonames: geonames.getTopLevel(),
		},
		EventDate: time.Now().String(),
		source:    "getGeonamesFileFromBucket",
	}

	return nil
}

func getLocalizationFileFromBucket(minioClient *minio.Client, objKey string, objDate time.Time, formattedFilename string, targetImg string) error {
	filePath := filepath.Join(config.mainCacheDir, formattedFilename)

	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}

	if timer, found := timers[formattedFilename]; found {
		timer.Stop()
	}

	localization, err := parseLocalization(filePath, objDate)
	if err != nil {
		return err
	}

	img, found := mainCache.findImageByPrefix(targetImg)
	if found {
		img.AssociatedLocalization = &localization
	}

	localizationCacheMutex.Lock()
	localizationCache[formattedFilename] = localization
	localizationCacheMutex.Unlock()
	timersMutex.Lock()
	timers[formattedFilename] = time.AfterFunc(config.RetentionPeriod, func() {
		localizationCacheMutex.Lock()
		delete(localizationCache, formattedFilename)
		localizationCacheMutex.Unlock()
		deleteFileFromCache(formattedFilename)
		timersMutex.Lock()
		delete(timers, formattedFilename)
		timersMutex.Unlock()
	})

	timersMutex.Unlock()

	return nil
}

func getFeaturesFileFromBucket(minioClient *minio.Client, objKey string, objDate time.Time, formattedFilename string, targetImg string, eventChan chan event) error {
	filePath := filepath.Join(config.mainCacheDir, formattedFilename)

	err := getFileFromBucket(minioClient, objKey, filePath)
	if err != nil {
		return err
	}

	if timer, found := timers[formattedFilename]; found {
		timer.Stop()
	}

	features, err := parseFeatures(filePath, objDate)
	if err != nil {
		return err
	}

	img, found := mainCache.findImageByPrefix(targetImg)
	if found {
		img.AssociatedFeatures = &features
	}

	featuresCacheMutex.Lock()
	featuresCache[formattedFilename] = features
	featuresCacheMutex.Unlock()
	timersMutex.Lock()
	timers[formattedFilename] = time.AfterFunc(config.RetentionPeriod, func() {
		featuresCacheMutex.Lock()
		delete(featuresCache, formattedFilename)
		featuresCacheMutex.Unlock()
		deleteFileFromCache(formattedFilename)
		timersMutex.Lock()
		delete(timers, formattedFilename)
		timersMutex.Unlock()
	})

	timersMutex.Unlock()
	eventChan <- event{
		EventType: eventFeatures,
		EventObj: EventFeatures{
			ImgKey:   targetImg,
			Class:    features.Class,
			Count:    features.Count,
			Features: features.Objects,
		},
		EventDate: time.Now().String(),
		source:    "getGeonamesFileFromBucket",
	}

	return nil
}

func deleteFileFromCache(fileName string) {
	err := os.Remove(filepath.Join(config.mainCacheDir, fileName))
	if err != nil && !os.IsNotExist(err) {
		printError(fmt.Errorf("failed to delete file from cache: %w", err), false)
	}

	printDebug("Removed", fileName, "from cache")
}

func existsInCache(imgName string, obj minio.ObjectInfo) (exists, needsUpdate bool) {
	if img, found := mainCache.findImageByKey(imgName); found {
		lastModTime := img.LastModified
		if obj.LastModified.Before(lastModTime) || obj.LastModified.Equal(lastModTime) {
			return true, false
		}

		printDebug("Found updated image: ", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))

		return true, true
	}

	printDebug("Found new image: ", fmt.Sprintf("%s (%.3fMB)", obj.Key, float64(obj.Size)/1e6))

	return false, false
}

func fetchThumbnailsFrom(imgDir, imgKey string, minioClient *minio.Client) []string {
	printInfo("Fetching thumbnails from ", imgDir, " ...")

	ctx, cancel := context.WithTimeout(context.Background(), config.PollingPeriod)
	defer cancel()

	thumbnails := make([]string, 0)
	s3ImgDir := strings.ReplaceAll(imgDir, "@", "/")

	for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{
		Prefix:    s3ImgDir,
		Recursive: true,
	}) {
		if !strings.HasSuffix(obj.Key, config.PreviewFilename) || obj.Key == imgKey {
			continue
		}

		formattedFilename := formatFileName(obj.Key)
		if _, found := thumbnailsCache.findImageByKey(obj.Key); !found {
			err := getImageFromBucket(thumbnailsCache, minioClient, obj.Key, formattedFilename, "", obj.LastModified, nil, false)
			if err != nil {
				printError(fmt.Errorf("failed to fetch thumbnail %q: %w", obj.Key, err), false)
				continue
			}

			thumbnailsCache.addImage(obj.Key, obj.Size, obj.LastModified)
		}

		thumbnails = append(thumbnails, getThumbnailsCacheFileLink(formattedFilename))
	}

	return thumbnails
}

func listMetaFiles(minioClient *minio.Client, dirs map[string]string, eventChan chan event) {
	printDebug(fmt.Sprintf("Looking for metadata files in bucket [%s] ...", config.S3.BucketName))

	tempFullProductLinksCache := map[string][]string{}

	for dir, targetImg := range dirs {
		func() { // usage of an anonymous function to call defer funcs at the end of each loop
			tempFullProductLinksCache[dir] = make([]string, 0)

			ctx, cancel := context.WithTimeout(context.Background(), config.PollingPeriod)
			defer cancel()

			printDebug("Looking for metadata files in ", dir)

			for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: dir + "/", Recursive: true}) {
				if obj.Err != nil {
					continue
				}

				// geonames
				if len(config.GeonamesFilename) > 0 && strings.HasSuffix(obj.Key, "/"+config.GeonamesFilename) { //nolint:nestif
					formattedFilename := formatFileName(dir + "/" + config.GeonamesFilename)
					if geonames, alreadyInCache := geonamesCache[formattedFilename]; alreadyInCache {
						if geonames.lastUpdate.Before(obj.LastModified) {
							err := getGeonamesFileFromBucket(minioClient, obj.Key, obj.LastModified, formattedFilename, targetImg, eventChan)
							if err != nil {
								printError(err, false)
								continue
							}
						}

						tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getMainCacheFileLink(strings.ReplaceAll(dir, "/", "@"), config.GeonamesFilename))

						continue
					}

					printDebug("Found geonames file: ", obj.Key)

					err := getGeonamesFileFromBucket(minioClient, obj.Key, obj.LastModified, formattedFilename, targetImg, eventChan)
					if err != nil {
						printError(err, false)

						continue
					}

					tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getMainCacheFileLink(strings.ReplaceAll(dir, "/", "@"), config.GeonamesFilename))

					continue
				}

				// localization
				if len(config.LocalizationFilename) > 0 && strings.HasSuffix(obj.Key, "/"+config.LocalizationFilename) {
					formattedFilename := formatFileName(dir + "/" + config.LocalizationFilename)
					if localizationCache[formattedFilename].lastUpdate.Before(obj.LastModified) {
						printDebug("Found localization file: ", obj.Key)

						err := getLocalizationFileFromBucket(minioClient, obj.Key, obj.LastModified, formattedFilename, targetImg)
						if err != nil {
							printError(err, false)

							continue
						}
					}
				}

				// features
				if config.featuresExtensionRegexp != nil && config.featuresExtensionRegexp.MatchString(obj.Key) {
					parts := strings.Split(obj.Key, "/")
					filename := parts[len(parts)-1]
					formattedFilename := formatFileName(dir + "/" + filename)

					if featuresCache[formattedFilename].lastUpdate.Before(obj.LastModified) {
						printDebug("Found features file: ", obj.Key)

						err := getFeaturesFileFromBucket(minioClient, obj.Key, obj.LastModified, formattedFilename, targetImg, eventChan)
						if err != nil {
							printError(err, false)
						}
					}

					tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getMainCacheFileLink(strings.ReplaceAll(dir, "/", "@"), filename))

					continue
				}

				// full product images
				if len(config.FullProductExtension) > 0 && strings.HasSuffix(obj.Key, config.FullProductExtension) {
					tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getFullProductImageLink(minioClient, obj.Key))

					continue
				}

				if config.additionalProductFilesRegexp != nil && config.additionalProductFilesRegexp.MatchString(obj.Key) {
					parts := strings.Split(obj.Key, "/")
					filename := parts[len(parts)-1]
					formattedFilename := formatFileName(dir + "/" + filename)

					if additionalProductFilesCache[formattedFilename].Before(obj.LastModified) {
						printDebug("Found additional product file: ", obj.Key)

						err := getFileFromBucket(minioClient, obj.Key, filepath.Join(config.mainCacheDir, formattedFilename))
						if err != nil {
							printError(err, false)

							continue
						}
					}

					additionalProductFilesCache[formattedFilename] = obj.LastModified

					tempFullProductLinksCache[dir] = append(tempFullProductLinksCache[dir], getMainCacheFileLink(strings.ReplaceAll(dir, "/", "@"), filename))
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

	printInfo(fmt.Sprintf("Looking for images in bucket [%s] ...", config.S3.BucketName))

	previewBaseDirs := map[string]string{}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	for _, imgType := range config.imageTypes {
		for obj := range minioClient.ListObjects(ctx, config.S3.BucketName, minio.ListObjectsOptions{Prefix: imgType.ProductPrefix, Recursive: true}) {
			if obj.Err != nil {
				handleS3Error(fmt.Errorf("no connection to S3 server => exit: %w", obj.Err))
				return obj.Err
			}

			if !imgType.productRegexp.MatchString(strings.TrimPrefix(obj.Key, imgType.ProductPrefix)) {
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
				img, found := mainCache.findImageByKey(obj.Key)
				if found && img.LastModified.Equal(obj.LastModified) {
					continue
				}
			}

			err := getImageFromBucket(mainCache, minioClient, obj.Key, formattedName, imgType.Name, obj.LastModified, eventChan, needsUpdate)
			if err != nil {
				return err
			}
			// As getImageFromBucket does not add the image to the mainCache,
			// we need to update it if the image was already there or add it manually if it's a new one
			img, found := mainCache.findImageByKey(obj.Key)
			if found {
				img.LastModified = obj.LastModified
			} else {
				mainCache.addImage(obj.Key, obj.Size, obj.LastModified)
			}
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

			err := extractFilesFromBucket(minioClient, eventChan)
			if err != nil {
				printError(fmt.Errorf("failed to extract files from bucket: %w", err), false)
			}
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
					printError(fmt.Errorf("failed to receive preview notification: %w", err), false)

					continue
				}

				for _, e := range notif.Records {
					obj := e.S3.Object
					objKey := obj.Key
					formattedName := formatFileName(objKey)

					if strings.HasPrefix(e.EventName, "s3:ObjectCreated") {
						printDebug("[Created]: ", objKey)

						err := getImageFromBucket(mainCache, minioClient, objKey, formattedName, inferImageType(objKey).Name, time.Now(), nil, false)
						if err != nil {
							printError(err, false)

							continue
						}

						objDate, err := time.Parse("2006-01-02T15:04:05.000Z", e.EventTime)
						if err != nil {
							printWarn(fmt.Errorf("failed to parse event time: %w", err), false)

							objDate = time.Now()
						}

						mainCache.addImage(objKey, obj.Size, objDate)
						eventChan <- event{EventType: eventAdd, EventObj: EventObject{
							ImgType: inferImageType(formattedName).Name,
							ImgKey:  formattedName,
							ImgName: getGeoname(formattedName),
							ImgDate: objDate.In(time.Local).Format("2006-01-02 15:04:05 MST"), //nolint:gosmopolitan
						}, EventDate: time.Now().String(),
							source: "listenToBucket"}
					} else if strings.HasPrefix(e.EventName, "s3:ObjectRemoved") {
						printDebug("[Removed]: ", objKey)
						deleteFileFromCache(formattedName)
						mainCache.deleteImage(formattedName)
						eventChan <- event{EventType: eventRemove, EventObj: EventObject{ImgKey: formattedName}, source: "listenToBucket"}
					}
				}
			case notif := <-geonamesNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive geonames notification: %w", err), false)

					continue
				}

				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					printDebug("[Created geonames]: ", objKey)

					img, found := mainCache.findImageByKey(objKey)
					if !found {
						continue
					}

					log.Println("Event time:", e.EventTime)
					//                          2016–09–08T22:34:38.226Z
					objDate, err := time.Parse("2006-01-02T15:04:05.000Z", e.EventTime)
					if err != nil {
						printError(fmt.Errorf("failed to parse event time: %w", err), false)

						objDate = time.Now()
					}

					err = getGeonamesFileFromBucket(minioClient, objKey, objDate, img.getAssociatedGeonamesPath(), img.FormattedKey, eventChan)
					if err != nil {
						printError(err, false)

						continue
					}
				}
			case notif := <-fullProductNotifs:
				if err := notif.Err; err != nil {
					printError(fmt.Errorf("failed to receive full product notification: %w", err), false)

					continue
				}
			RecordsLoop:
				for _, e := range notif.Records {
					objKey := e.S3.Object.Key
					printDebug("[Created full prod]: ", objKey)
					// formattedFilename := strings.ReplaceAll(objKey, "/", "@")
					// img, found := getCorrespondingImage(formattedFilename)
					img, found := mainCache.findImageByKey(objKey)
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
