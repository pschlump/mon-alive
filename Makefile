
all:
	( cd lib ; go build )
	( cd middleware ; go build )
	( cd cli ; go build )
	
