package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

/*

3. Add in notification destination and action for down items

Notes:
	Dir, _ = os.Getwd()

*/

var Debug = flag.Bool("debug", false, "Debug flag")                       // 0
var Cfg = flag.String("cfg", "../global_cfg.json", "Configuration file")  // 1
var LoadFn = flag.String("load", "", "Configuration file to load")        // 2
var DumpFn = flag.String("dump", "", "Dump configuration to file")        // 3
var Verbose = flag.Bool("verbose", false, "verbose output")               // 4
var Periodic = flag.String("periodic", "", "loop forever showing output") // 5
func init() {
	flag.BoolVar(Debug, "D", false, "Debug flag")                              // 0
	flag.StringVar(Cfg, "c", "../global_cfg.json", "Configuration file")       // 1
	flag.StringVar(LoadFn, "l", "", "Configuration file to load")              // 2
	flag.StringVar(DumpFn, "d", "", "Dump configuration to file to listen to") // 3
	flag.BoolVar(Verbose, "v", false, "verbose output")                        // 4
	flag.StringVar(Periodic, "P", "", "loop forever showing output")           // 5
}

func main() {

	flag.Parse()
	fns := flag.Args()

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
		fmt.Printf("Loaded: OK\n")
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

	// config-info
	// conn.Cmd("SREM", "monitor:potentialItem", itemName) // Actually monitoring this item
	// conn.Cmd("SADD", "monitor:IAmAlive", itemName)

	for ii := 0; ii < len(fns); ii++ {
		cmd := fns[ii]
		// fmt.Printf("Running >%s<\n", cmd)
		switch cmd {
		case "i-am-alive":
			myStatus := make(map[string]interface{})
			myStatus["cli"] = "y"
			// fmt.Printf("Sending IAmAlive to [%s], %s\n", fns[ii+1], godebug.LF())
			mon.SendIAmAlive(fns[ii+1], myStatus)
			ii++
		case "status":

			showStatus := func() {
				st := mon.GetStatusOfItemVerbose(*Verbose)
				// fmt.Printf("%s\n", lib.SVarI(st))
				fmt.Printf("%4s  %-30s %-5s %-30s\n", "", "Name", "Stat.", "Data")
				fmt.Printf("%5s %-30s %-5s %-30s\n", "-----", "------------------------------", "-----", "-------------------------")
				for ii, vv := range st {
					if vv.Status == "up" {
						fmt.Printf("%4d: %-30s %s%-5s%s %-30s\n", ii, vv.Name, MiscLib.ColorGreen, vv.Status, MiscLib.ColorReset, vv.Data)
					} else {
						fmt.Printf("%4d: %-30s %s%-5s%s %-30s\n", ii, vv.Name, MiscLib.ColorRed, vv.Status, MiscLib.ColorReset, vv.LongName)
					}
				}
			}

			if *Periodic != "" {
				nSec, err := strconv.ParseInt(*Periodic, 10, 64)
				if err != nil {
					fmt.Printf("Error: %s converting [%s] number of seconds, assuming 60\n", err, *Periodic)
					nSec = 60
				}
				for {
					fmt.Printf("\n")
					showStatus()
					time.Sleep(time.Duration(1000*nSec) * time.Millisecond)
				}
			} else {
				showStatus()
			}

		}
	}

}
