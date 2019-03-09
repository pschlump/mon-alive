
all:
	go build

run: file_output

test: all test1

test1:
	./mon-cli load --file ./cfg0.json
	./mon-cli dump --file ,tmp.out
	diff -w ,tmp.out ./cfg0.json

install:
	cp mon-cli ~/bin

linux:
	GOOS=linux go build -o mon-cli.linux .

load_config:
	./mon-cli load --file ./cfg0.json
	
dump_config:
	./mon-cli dump --file -

up_down:
	./mon-cli status -P 30

new_up_down:
	./mon-cli live-monitor

file_output:
	./mon-cli live-monitor --quiet --file /Users/corwin/go/src/github.com/pschlump/Go-FTL/server/goftl/testdata/9001/www/lm/data.json 2>,log.out &

cp_to_linux_sys:
	scp mon-cli.linux *.tmpl pschlump@192.168.0.140:/home/pschlump/bin

