
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
	
load_prod_config:
	./mon-cli.linux --cfg prod-global-cfg.json load --file ./prod-cfg0.json

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

deploy_74:
	echo GOOS=linux go build -o mon-cli.linux .
	echo ssh pschlump@192.154.97.74 "mkdir -p ./tools/mon-cli"
	-echo ssh pschlump@192.154.97.74 "mv ./tools/mon-cli/mon-cli.linux ,aaaaa"
	echo scp *.linux pschlump@192.154.97.74:/home/pschlump/tools/mon-cli
	scp prod-cfg0.json LICENSE pschlump@192.154.97.74:/home/pschlump/tools/mon-cli
	echo scp Makefile pschlump@192.154.97.74:/home/pschlump/tools/mon-cli
	echo scp prod-global-cfg.json pschlump@192.154.97.74:/home/pschlump/tools/mon-cli


