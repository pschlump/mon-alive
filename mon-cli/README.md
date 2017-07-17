mon-cli : command line monitor / tracer version
===============================================

## ToDo

1. Convert the configuration to a "set" of configs in Redis - simpler to update.
2. Add in xyzzyAddCRUD - All the indifidual configs
3. Add in a "PUBLISH" to send this to a socket.io listener for clients on the web?? - may do this as a bot and
	use this code as a library.
4. 


## Note

A return status of 404 for not found on a pinged server is considered to be a UP server!
This means that you got *A* response from the server - you do not need to get a good 
response for the server to be up.

This may change with an option for "set of valid status codes" and "set of RE matches" 
that are a UP server.











To load a configuraiton

```bash

	$ ./mon-cli load -f cfg0.json

```

To dump to screen the current configuration

```bash

	$ ./mon-cli dump

```

