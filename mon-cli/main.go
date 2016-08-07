package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pschlump/MiscLib"
	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
	"github.com/urfave/cli"
)

/*

3. Add in notification destination and action for down items

Notes:
	Dir, _ = os.Getwd()

*/

func main() {
	app := cli.NewApp()
	app.Name = "mon-cli"
	app.Usage = "Tracer/Live monitor - CLI version"
	app.Version = "0.5.9"

	type commonConfig struct {
		MyStatus map[string]interface{} //
		Name     string                 //
		Debug    map[string]bool        // make this a map[string]bool set of flags that you can turn on/off
		conn     *redis.Client          //
		mon      *MonAliveLib.MonIt     //
	}

	cc := commonConfig{
		MyStatus: make(map[string]interface{}),
		Name:     "mon-alive",
		Debug:    make(map[string]bool),
	}
	cc.MyStatus["cli"] = "y"

	app.Before = func(c *cli.Context) error {

		DebugFlags := c.GlobalString("debug")
		ds := strings.Split(DebugFlags, ",")
		for _, dd := range ds {
			cc.Debug[dd] = true
		}

		// do setup - common function -- Need to be able to skip for i-am-alive remote!
		cfg := c.GlobalString("cfg")
		qdemolib.SetupRedisForTest(cfg)
		connTmp, conFlag := qdemolib.GetRedisClient()
		if !conFlag {
			fmt.Printf("Did not connect to redis\n")
			os.Exit(1)
		}
		cc.conn = connTmp

		monTmp := MonAliveLib.NewMonIt(func() *redis.Client { return cc.conn }, func(conn *redis.Client) {})
		cc.mon = monTmp

		return nil
	}

	create_IAmAlive := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			// xyzzy - handle remote at this spoint! -- this is doing a "get" with API Key on a remote server to signal that you are alive [ No Redis ]

			cc.Name = ctx.String("name")

			// xyzzy - get "status, S" at this point // xyzzy - add in addiitonal status

			cc.mon.SendIAmAlive(cc.Name, cc.MyStatus)
			if cc.Debug["show-feedback"] {
				fmt.Printf("I Am Alive sent for [%s]: OK\n", cc.Name)
			}
			return nil
		}
	}

	create_Load := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			LoadFn := ctx.String("file")
			cc.mon.SetConfigFromFile(LoadFn)
			if cc.Debug["show-feedback"] {
				fmt.Printf("Loaded: OK\n")
			}
			return nil
		}
	}

	create_Dump := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			DumpFn := ctx.String("file")
			s := cc.mon.GetConfig()
			if DumpFn == "-" || DumpFn == "" {
				fmt.Printf("%s\n", s)
			} else {
				err := ioutil.WriteFile(DumpFn, []byte(s+"\n"), 0600)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s writing %s\n", err, DumpFn)
					os.Exit(1)
				}
				if cc.Debug["show-feedback"] {
					fmt.Printf("Dumped to %s: OK\n", DumpFn)
				}
			}
			return nil
		}
	}

	create_Status := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			Verbose := ctx.Bool("verbose")
			Periodic := ctx.String("periodic")

			showStatus := func() {
				st := cc.mon.GetStatusOfItemVerbose(Verbose)
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

			if Periodic != "" {
				nSec, err := strconv.ParseInt(Periodic, 10, 64)
				if err != nil {
					fmt.Printf("Error: %s converting [%s] number of seconds, assuming 60\n", err, Periodic)
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
			return nil
		}
	}

	create_Trace := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			return nil
		}
	}

	app.Commands = []cli.Command{
		{
			Name:   "i-am-alive",
			Usage:  "Report to the monitor that you are alive.",
			Action: create_IAmAlive(),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "name to report it is alive",
				},
				cli.StringFlag{ // xyzzy - not implemented yet
					Name:  "remote, R",
					Usage: "URL to use to report that you are alive - remote reporing",
				},
				cli.StringFlag{ // xyzzy - not implemented yet
					Name:  "status, S",
					Usage: "Additional status to report",
				},
			},
		},
		{
			Name:   "load",
			Usage:  "Load a new configuration for the monitor from a file",
			Action: create_Load(),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Usage: "name of file to load",
				},
			},
		},
		{
			Name:   "dump",
			Usage:  "Print the currenlty loaded configuration",
			Action: create_Dump(),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Usage: "name of output file to print to, \"-\" is stdout.",
				},
			},
		},
		{
			Name:   "status",
			Usage:  "Show the up/down status of monitored processes",
			Action: create_Status(),
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "periodic, P",
					Usage: "Set the frequency of displaying and run in a loop forever.",
				},
			},
		},
		{
			Name:   "trace",
			Usage:  "Trace calls to the server",
			Action: create_Trace(),
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "trx-id, T",
					Usage: "Trace a specific client.",
				},
				cli.StringFlag{
					Name:  "periodic, P",
					Usage: "Set the frequency of displaying and run in a loop forever.",
				},
			},
		},
		// xyzzy - list non-users (anonomous / not logged in folks)
		// xyzzy - list users logged in
		// xyzzy - watch all queries
		// xyzzy - watch all requests
		// xyzzy - get load levels
		// xyzzy - start new service
		// xyzzy - stop service
		// xyzzy - set notification destination
		// xyzzy - set notification conditions
		// xyzzy - set actions and conditions to take actions (start/stop microserice, servers, etc)
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "cfg, c",
			Value: "../global_cfg.json",
			Usage: "Global Configuration File.",
		},
		cli.StringFlag{
			Name:  "debug, D",
			Value: "",
			Usage: "Set debug flags [ show-feedback ]",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
