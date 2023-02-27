#!/bin/bash -eu

OPENLAYERS_JS_URL="https://cdn.jsdelivr.net/npm/ol@latest/dist/ol.js"
OPENLAYERS_JS_MAP_URL="https://cdn.jsdelivr.net/npm/ol@latest/dist/ol.js.map"
OPENLAYERS_CSS_URL="https://cdn.jsdelivr.net/npm/ol@latest/ol.css"
OPENLAYERS_DIR="src/resources/vendor/openlayers"

echo "Updating Go dependencies ..."
go get -u ./... && go mod tidy

echo "Updating OpenLayers dependencies ..."
mkdir -p "$OPENLAYERS_DIR"
wget $OPENLAYERS_JS_URL -O "$OPENLAYERS_DIR/ol.js"
wget $OPENLAYERS_JS_MAP_URL -O "$OPENLAYERS_DIR/ol.js.map"
wget $OPENLAYERS_CSS_URL -O "$OPENLAYERS_DIR/ol.css"

echo "Done !"
