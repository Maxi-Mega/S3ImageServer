# S3ImageServer [V2.0.1]

### Browse images from S3 bucket

## Starting the server

```bash
./S3ImageServer-x.y.z -c config.yml
```

Where `config.yml` is the path to the configuration file

## Configuration file example

```yaml
s3:
    endPoint: "192.168.0.27:9000"
    bucketName: "my-bucket"
    keyPrefix: "my-prefix"
    accessId: "admin"
    accessSecret: "password"
    useSSL: false                 # Not tested

basePath: "" # Empty or starting with a slash
windowTitle: "S3 Image Viewer"
scaleInitialPercentage: 50
previewFilename: "preview.jpg"
geonamesFilename: "geonames.json"
fullProductExtension: "tif"
fullProductProtocol: "protocol://"
fullProductSignedUrl: false
imageTypes:
    - "TYPE1"
    - "TYPE2"
    - "TYPE3"

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
