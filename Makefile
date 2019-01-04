
all:
	( cd lib ; go build )
	( cd middleware ; go build )
	( cd cli ; go build )

test:	
	( cd lib ; go test )
	( cd middleware ; go test )

run:
	( cd /Users/corwin/go/src/github.com/pschlump/mon-alive/mon-cli ; make file_output )

