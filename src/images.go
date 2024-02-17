package main

import (
	"io/fs"
	"sort"
	"strings"
	"time"
)

type S3Image struct {
	S3Key        string
	LastModified time.Time
	Size         int64

	FormattedKey string
	// PathOnDisk   string

	Type                   *ImageType
	AssociatedGeonames     *Geonames
	AssociatedLocalization *Localization
	AssociatedFeatures     *Features
}

func newS3ImageFromCache(imagePath string, fileInfo fs.FileInfo) S3Image {
	s3Path := strings.TrimPrefix(strings.ReplaceAll(imagePath, "@", "/"), "/")

	return S3Image{
		S3Key:        s3Path,
		LastModified: fileInfo.ModTime(),
		Size:         fileInfo.Size(),
		FormattedKey: strings.ReplaceAll(s3Path, "/", "@"),
		// PathOnDisk:         imagePath,
		Type:               inferImageType(s3Path),
		AssociatedGeonames: nil,
	}
}

func inferImageType(imageName string) *ImageType {
	imageName = strings.ReplaceAll(imageName, "@", "/")
	for _, imgType := range config.imageTypes {
		if strings.HasPrefix(imageName, imgType.ProductPrefix) {
			return &imgType
		}
	}

	printDebug("Image type for image '" + imageName + "' not found !")

	return nil
}

func (image S3Image) getAssociatedGeonamesPath() string {
	return image.FormattedKey[:strings.LastIndex(image.FormattedKey, "@")+1] + config.GeonamesFilename
}

func (image S3Image) String() string {
	return image.S3Key
}

type ImageCache struct {
	pathOnDisk string
	images     []S3Image
}

func (images *ImageCache) findImageByKey(key string) (image *S3Image, found bool) {
	for i, img := range images.images {
		if img.S3Key == key {
			return &images.images[i], true
		}
	}

	return nil, false
}

func (images *ImageCache) findImageByPrefix(prefix string) (image *S3Image, found bool) {
	for i, img := range images.images {
		if strings.HasPrefix(img.S3Key, prefix) {
			return &images.images[i], true
		}
	}

	return nil, false
}

func (images *ImageCache) toEventObjects() []EventObject {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	sort.Slice(images.images, func(i, j int) bool {
		return images.images[i].LastModified.After(images.images[j].LastModified) // Usage of After to invert the sort order
	})

	maxImagesCount := len(images.images)
	if maxImagesCount > config.MaxImagesDisplayCount {
		maxImagesCount = config.MaxImagesDisplayCount
	}

	result := make([]EventObject, maxImagesCount)

	for i, image := range images.images {
		if i >= maxImagesCount {
			// convert only the maxImagesCount first images,
			// maxImagesCount being the minimum between the number of available images and the max images display count
			break
		}

		features := Features{}

		if image.AssociatedFeatures != nil {
			/*for feature, count := range *image.AssociatedFeatures {
				features += fmt.Sprintf("%s: %d ", feature, count)
			}*/
			features = *image.AssociatedFeatures
		}

		result[i] = EventObject{
			ImgType:  image.Type.Name,
			ImgKey:   image.FormattedKey,
			ImgName:  getGeoname(image.FormattedKey),
			ImgDate:  image.LastModified.In(time.Local).Format("2006-01-02 15:04:05 MST"),
			Features: features,
		}
	}

	return result
}

func (images *ImageCache) addImage(objKey string, size int64, lastModified time.Time) {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	images.images = append(images.images, S3Image{
		S3Key:        objKey,
		LastModified: lastModified,
		Size:         size,
		FormattedKey: strings.ReplaceAll(objKey, "/", "@"),
		// PathOnDisk:         "",
		Type:               inferImageType(objKey),
		AssociatedGeonames: nil,
	})
}

func (images *ImageCache) deleteImage(formattedName string) {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	for i, img := range images.images {
		if img.FormattedKey == formattedName {
			images.images = append(images.images[:i], images.images[i+1:]...)

			return
		}
	}
}
