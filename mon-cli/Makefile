
all:
	go build

test: all test1

test1:
	./cli -l ./cfg0.json
	./cli -d tmp.out
	diff tmp.out ./cfg0.json

install:
	cp cli ~/bin/mon-alive-cli

