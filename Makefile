
all:
	( cd lib ; go build )
	( cd middleware ; go build )
	( cd cli ; go build )

test:	
	( cd lib ; go test )
	( cd middleware ; go test )

