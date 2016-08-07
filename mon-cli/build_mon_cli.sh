#!/bin/bash

cd 
mkdir -p ./go/src/github.com/pschlump/mon-alive/mon-cli

#cd go/src/github.com/pschlump/mon-alive/mon-cli

cd go/src/github.com/pschlump/mon-alive
tar -xf ~/x.tar 
cd ./mon-cli
go get
make

mv mon-cli mon-cli.linux
tar -czf ~/x.tar.gz ./mon-cli.linux

