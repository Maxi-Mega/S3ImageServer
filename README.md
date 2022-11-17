# S3ImageServer [V3.0.0-SNAPSHOT]

### Browse images from S3 bucket

## Starting the server

```bash
./S3ImageServer-x.y.z -c config.yml
```

Where `config.yml` is the path to the configuration file

## Configuration file example

```yaml
s3:
  endPoint: "127.0.0.1:9000"
  bucketName: "my-bucket"
  accessId: "admin"
  accessSecret: "password"
  useSSL: false                 # Not tested

basePath: "" # Empty or starting with a slash
windowTitle: "S3 Image Viewer"
scaleInitialPercentage: 50
previewFilename: "preview.jpg"
geonamesFilename: "geonames.json"
featuresExtensionRegexp: "sample.*\\.json$"
featuresPropertyName: "detection"
fullProductExtension: "tif"
fullProductProtocol: "protocol://"
fullProductRootUrl: "http://a.b.c.d:5000"
fullProductSignedUrl: false
imageGroups:
  - groupName: "Group 1"
    types:
      - name: "TYPE1"
        displayName: "Type 1"
        productPrefix: "my-prefix/TYPE1/"
        productRegexp: "^(?P<parent>.*/DIR_[^/]*/[^/]*)/preview.jpg$"
      - name: "TYPE2"
        displayName: "Type 2"
        productPrefix: "my-prefix/TYPE2/"
        productRegexp: "^(?P<parent>.*/DIR_[^/]*/[^/]*)/preview.jpg$"
  - groupName: "Group 2"
    types:
      - name: "TYPE3"
        displayName: "Type 3"
        productPrefix: "my-prefix/TYPE3/"
        productRegexp: "^(?P<parent>.*/DIR_[^/]*/[^/]*)/preview.jpg$"

logLevel: "info"
colorLogs: false
jsonLogFormat: false
jsonLogFields:
  class_name: "prod"
  server: 42
httpTrace: false
exitOnS3Error: false
cacheDir: ""        # Nothing = default
retentionPeriod: 10m
maxImagesDisplayCount: 10
pollingMode: false
pollingPeriod: 30s
webServerPort: 9999
```

## Build

Go to the `src` directory and execute:

### - For a dynamic excutable:

```bash
go build -o S3ImageViewer
```

### - For a static excutable:

```bash
go build -ldflags="-extldflags=-static" -tags osusergo,netgo -o S3ImageViewer
```
