#!/bin/bash -eu

echo "Updating dependencies ..."

go get -u ./... && go mod tidy

if [ $? -eq 0 ]; then
  echo "Done !"
else
  echo "Failed to update dependencies"
fi
