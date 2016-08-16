
all:
	go build

test: all test1

test1:
	./mon-cli load --file ./cfg0.json
	./mon-cli dump --file ,tmp.out
	diff ,tmp.out ./cfg0.json

install:
	cp mon-cli ~/bin

linux:
	( cd .. ; tar -cf ~/x.tar . )
	scp ~/x.tar build_mon_cli.sh pschlump@192.168.0.182:/home/pschlump
	ssh pschlump@192.168.0.182 chmod +x ./build_mon_cli.sh
	ssh pschlump@192.168.0.182 ./build_mon_cli.sh
	scp pschlump@192.168.0.182:/home/pschlump/x.tar.gz .
	tar -xzf x.tar.gz
	
load_config:
	./mon-cli load --file ./cfg0.json
	
dump_config:
	./mon-cli dump --file -

up_down:
	./mon-cli status -P 30

cp_to_linux_sys:
	scp mon-cli.linux *.tmpl pschlump@192.168.0.140:/home/pschlump/bin

