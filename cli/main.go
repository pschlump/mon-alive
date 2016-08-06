package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

/*

1. A CLI that will run a command and do stuff

	1. I AM Alive - send message for somebody -- Perform "GET"

cli i-am-alive Name

cli status
	Name Up/Dn
	Name Up/Dn
	Name Up/Dn
	Name Up/Dn

cli -v status
	Name Up/Dn LoadLevel SelfReport/PokeIt  TimeTillEvent

cli -j -v status -- Saem output in JSON format


-l CfgFile		-- default to mon-alive.json


*/

var Debug = flag.Bool("debug", false, "Debug flag")                                // 0
var Cfg = flag.String("cfg", "../global_cfg.json", "Configuration file")           // 1
var LoadFn = flag.String("load", "./mon-alive.json", "Configuration file to load") // 2
var DumpFn = flag.String("dump", "", "Dump configuration to file")                 // 3
var Verbose = flag.String("verbose", "", "verbose output")                         // 4
var Periodic = flag.String("periodic", "", "loop forever showing output")          // 5
func init() {
	flag.BoolVar(Debug, "D", false, "Debug flag")                                 // 0
	flag.StringVar(Cfg, "c", "../global_cfg.json", "Configuration file")          // 1
	flag.StringVar(LoadFn, "l", "./mon-alive.json", "Configuration file to load") // 2
	flag.StringVar(DumpFn, "d", "", "Dump configuration to file to listen to")    // 3
	flag.StringVar(Verbose, "v", "", "verbose output")                            // 4
	flag.StringVar(Periodic, "P", "", "loop forever showing output")              // 5
}

func main() {

	flag.Parse()
	fns := flag.Args()

	if *DumpFn != "" && *LoadFn != "" {
		fmt.Printf("Only one of --load --dump at a time\n")
		flag.Usage()
		os.Exit(1)
	}

	qdemolib.SetupRedisForTest(*Cfg)

	conn, conFlag := qdemolib.GetRedisClient()
	if !conFlag {
		fmt.Printf("Did not connect to redis\n")
		os.Exit(1)
	}

	mon := MonAliveLib.NewMonIt(func() *redis.Client { return conn }, func(conn *redis.Client) {})

	if *LoadFn != "" {
		// fmt.Printf("At: %s\n", godebug.LF())
		mon.SetConfigFromFile(*LoadFn)
	}
	if *DumpFn != "" {

		s := mon.GetConfig()
		if *DumpFn == "-" {
			fmt.Fprintf(os.Stdout, "%s\n", s)
		} else {
			err := ioutil.WriteFile(*DumpFn, []byte(s+"\n"), 0600)
			if err != nil {
				fmt.Printf("Error: %s writing %s\n", err, *DumpFn)
				return
			}
		}
	}

	for ii := 1; ii < len(fns); ii++ {
		cmd := fns[ii]
		switch cmd {
		case "i-am-alive":
			myStatus := make(map[string]interface{})
			myStatus["cli"] = "y"
			mon.SendIAmAlive(fns[ii+1], myStatus)
		case "status":
			// xyzzy - report on all (remember -v flag)
			/*
			   type ItemStatus struct {
			   	Name     string
			   	Status   string
			   	Data     string
			   	LongName string
			   }
			*/
			st := mon.GetStatusOfItemVerbose()
			fmt.Printf("%s\n", lib.SVarI(st))
		}
	}

}
