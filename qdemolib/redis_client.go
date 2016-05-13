package qdemolib

import (
	"log"

	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

func GetRedisClient() (client *redis.Client, conFlag bool) {
	var err error
	client, err = redis.Dial("tcp", qdemolib.ServerGlobal.RedisConnectHost+":"+qdemolib.ServerGlobal.RedisConnectPort)
	if err != nil {
		log.Fatal(err)
	}
	if qdemolib.ServerGlobal.RedisConnectAuth != "" {
		err = client.Cmd("AUTH", qdemolib.ServerGlobal.RedisConnectAuth).Err
		if err != nil {
			log.Fatal(err)
		} else {
			conFlag = true
		}
	} else {
		conFlag = true
	}
	return
}
