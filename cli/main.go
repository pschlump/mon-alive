package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

var Debug = flag.Bool("debug", false, "Debug flag")                      // 0
var Cfg = flag.String("cfg", "../global_cfg.json", "Configuration file") // 1
var LoadFn = flag.String("load", "", "Configuraiton file to load")       // 2
var DumpFn = flag.String("load", "", "Dump configration to file")        // 3
func init() {
	flag.BoolVar(Debug, "D", false, "Debug flag")                             // 0
	flag.StringVar(Cfg, "c", "", "Configuration file")                        // 1
	flag.StringVar(LoadFn, "l", "", "Configuraiton file to load")             // 2
	flag.StringVar(DumpFn, "d", "", "Dump configration to file to listen to") // 3
}

func RedisClient() (client *redis.Client, conFlag bool) {
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

func main() {

	flag.Parse()
	fns := flag.Args()
	if len(fns) != 0 {
		flag.Usage()
		os.Exit(1)
	}
	if *DumpFn != "" && *LoadFn != "" {
		fmt.Printf("Only one of --load --dump at a time\n")
		flag.Usage()
		os.Exit(1)
	}

	qdemolib.SetupRedisForTest(*Cfg)

	conn, conFlag := RedisClient()
	if !conFlag {
		// xyzzy - report error
	}

	mon := MonAliveLib.NewMonIt(func() (conn *redis.Client) { return conn }, func(conn *redis.Client) {})

	if *LoadFn != "" {
		mon.SetConfigFromFile(*LoadFn)
	}
	if *DumpFn != "" {

		s, err := conn.Cmd("GET", "monitor:config").Str()
		if err != nil {
			fmt.Printf("Error: %s seting configuration  - File: %s\n", err, *DumpFn)
			return
		}

		ioutil.WriteFile(*DumpFn, []byte(s), 0600)
		if err != nil {
			fmt.Printf("Error: %s writing %s\n", err, *DumpFn)
			return
		}
	}

}
