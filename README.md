# S3ImageServer [V1.3.2]
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

previewFilename: "preview.jpg"
imageTypes:
  - "TYPE1"
  - "TYPE2"
  - "TYPE3"

debug: false
httpTrace: false
cacheDir: ""        # Nothing = default
retentionPeriod: 10m
pollingMode: false
pollingPeriod: 30s
webServerPort: 9999
```
