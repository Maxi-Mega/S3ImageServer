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
	PathOnDisk   string

	Type               *ImageType
	AssociatedGeonames *Geonames
	AssociatedFeatures *Features
}

func newS3ImageFromCache(imagePath string, fileInfo fs.FileInfo) S3Image {
	pathWithoutParentDir := strings.ReplaceAll(strings.TrimPrefix(imagePath, config.CacheDir), "@", "/")
	pathWithoutParentDir = strings.TrimPrefix(pathWithoutParentDir, "/")
	return S3Image{
		S3Key:              pathWithoutParentDir,
		LastModified:       fileInfo.ModTime(),
		Size:               fileInfo.Size(),
		FormattedKey:       strings.ReplaceAll(pathWithoutParentDir, "/", "@"),
		PathOnDisk:         imagePath,
		Type:               inferImageType(pathWithoutParentDir),
		AssociatedGeonames: nil,
	}
}

func inferImageType(imageName string) *ImageType {
	imageName = strings.ReplaceAll(imageName, "@", "/")
	for _, imgType := range config.imageTypes {
		if strings.HasPrefix(imageName, imgType.Path) {
			return &imgType
		}
	}
	printDebug("Image type for image '" + imageName + "' not found !")
	return nil
}

func (image S3Image) getAssociatedGeonamesPath() string {
	return image.FormattedKey[:strings.LastIndex(image.FormattedKey, "@")+1] + config.GeonamesFilename
}

type S3Images []S3Image

func (images S3Images) findImageByKey(key string) (image *S3Image, found bool) {
	for i, img := range images {
		if img.S3Key == key {
			return &images[i], true
		}
	}
	return nil, false
}

func (images S3Images) findImageByPrefix(prefix string) (image *S3Image, found bool) {
	for i, img := range images {
		if strings.HasPrefix(img.S3Key, prefix) {
			return &images[i], true
		}
	}
	return nil, false
}

func (images S3Images) toEventObjects() []EventObject {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	sort.Slice(images, func(i, j int) bool {
		return images[i].LastModified.After(images[j].LastModified) // Usage of after to invert the sort order
	})

	maxImagesCount := len(images)
	if maxImagesCount > config.MaxImagesDisplayCount {
		maxImagesCount = config.MaxImagesDisplayCount
	}

	result := make([]EventObject, maxImagesCount)
	for i, image := range images {
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
			Features: features,
		}
	}

	/*if config.MaxImagesDisplayCount > 0 && len(result) > config.MaxImagesDisplayCount {
		return result[:config.MaxImagesDisplayCount] // keep only the n first images, n being the max images display count
	}*/

	return result
}

func (images *S3Images) addImage(objKey string, size int64, lastModified time.Time) {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	*images = append(*images, S3Image{
		S3Key:              objKey,
		LastModified:       lastModified,
		Size:               size,
		FormattedKey:       strings.ReplaceAll(objKey, "/", "@"),
		PathOnDisk:         "",
		Type:               inferImageType(objKey),
		AssociatedGeonames: nil,
	})
}

func (images *S3Images) deleteImage(formattedName string) {
	imagesCacheMutex.Lock()
	defer imagesCacheMutex.Unlock()

	for i, img := range *images {
		if img.FormattedKey == formattedName {
			*images = append((*images)[:i], (*images)[i+1:]...)
			return
		}
	}
}
