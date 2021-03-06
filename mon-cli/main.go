// Copyright (C) Philip Schlump, 2016-2017.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"
	"unsafe"

	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/Go-FTL/server/tr"
	"github.com/pschlump/MicroServiceLib"
	"github.com/pschlump/MiscLib"
	"github.com/pschlump/godebug"
	"github.com/pschlump/json"
	"github.com/pschlump/mon-alive/ListenLib"
	"github.com/pschlump/mon-alive/lib"
	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
	"github.com/urfave/cli"
)

/*

1. Issues
	+1.  xyzzyAddCRUD - CRUD on monitored items.


SETUP in Redis:

> config set notify-keyspace-events AKE

Misc Notes:
	Dir, _ = os.Getwd()

https://github.com/yaronsumel/grapes -- Remote execution of commands via SSH on sets of computers.
	-- this would be perfect for "ping" level 1 - to systems in live monitor
	-- also colud do stuff like "ps" and grep for running process



*/

// var message tr.Trx
// type Trx struct {
type TrxExtended struct {
	tr.Trx
	ColorRed     string
	ColorYellow  string
	ColorGreen   string
	ColorCyan    string
	ColorReset   string
	ScreenHeight int
	ScreenWidth  int
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func GetSize() (h uint, w uint) {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		panic(errno)
	}
	w = uint(ws.Col)
	h = uint(ws.Row)
	return
}

func main() {
	app := cli.NewApp()
	app.Name = "mon-cli"
	app.Usage = "Tracer/Live monitor - CLI version"
	app.Version = "0.6.0"

	type commonConfig struct {
		MyStatus map[string]interface{}     //
		Name     string                     //
		Debug    map[string]bool            // make this a map[string]bool set of flags that you can turn on/off
		conn     *redis.Client              //
		mon      *MonAliveLib.MonIt         //
		ms       *MicroServiceLib.MsCfgType //
		// initializedTrace bool                       //
	}

	cc := commonConfig{
		MyStatus: make(map[string]interface{}),
		Name:     "mon-alive",
		Debug:    make(map[string]bool),
	}
	cc.MyStatus["cli"] = "y"

	// fmt.Fprintf(os.Stderr, "%s Before app.Before , %s %s\n", MiscLib.ColorGreen, godebug.LF(), MiscLib.ColorReset)

	app.Before = func(c *cli.Context) error {

		// fmt.Fprintf(os.Stderr, "%s In app.Before , %s %s\n", MiscLib.ColorGreen, godebug.LF(), MiscLib.ColorReset)

		DebugFlags := c.GlobalString("debug")
		ds := strings.Split(DebugFlags, ",")
		for _, dd := range ds {
			cc.Debug[dd] = true
		}

		// do setup - common function -- Need to be able to skip for i-am-alive remote!
		cfg := c.GlobalString("cfg")
		qdemolib.SetupRedisForTest(cfg)
		// fmt.Fprintf(os.Stderr, "%s should have global setup, %s %s\n", MiscLib.ColorGreen, godebug.LF(), MiscLib.ColorReset)
		connTmp, conFlag := qdemolib.GetRedisClient()
		if !conFlag {
			fmt.Printf("Did not connect to redis\n")
			os.Exit(1)
		}
		cc.conn = connTmp

		monTmp := MonAliveLib.NewMonIt(func() *redis.Client { return cc.conn }, func(conn *redis.Client) {}, os.Stderr)
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

	//		Action: create_AddItem(), //  xyzzyAddCRUD - CRUD on monitored items.
	create_AddItem := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {

			key := ctx.String("key")
			name := ctx.String("name")
			ttlstr := ctx.String("ttl")
			ttl, err := strconv.ParseInt(ttlstr, 10, 64)
			if err != nil {
				ttl = 120
			}
			ping := ctx.Bool("ping")
			url := ctx.String("url")

			it := cc.mon.UpdateConfig()
			if _, ok := it.Item[key]; !ok {
				it.Item[key] = MonAliveLib.ConfigItem{
					Name:         name,
					TTL:          uint64(ttl),
					RequiresPing: ping,
					PingUrl:      url,
				}
				cc.mon.SetConfig(godebug.SVar(it))
			} else {
				fmt.Fprintf(os.Stderr, "Error: Unable to add %s - already exists - use `upd-item` instead?\n", key)
			}

			return nil
		}
	}

	create_UpdItem := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {

			key := ctx.String("key")
			name := ctx.String("name")
			ttlstr := ctx.String("ttl")
			ttl, err := strconv.ParseInt(ttlstr, 10, 64)
			if err != nil {
				ttl = 120
			}
			ping := ctx.Bool("ping")
			url := ctx.String("url")

			it := cc.mon.UpdateConfig()
			it.Item[key] = MonAliveLib.ConfigItem{
				Name:         name,
				TTL:          uint64(ttl),
				RequiresPing: ping,
				PingUrl:      url,
			}
			cc.mon.SetConfig(godebug.SVar(it))

			return nil
		}
	}

	create_RmItem := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {

			key := ctx.String("key")

			it := cc.mon.UpdateConfig()
			delete(it.Item, key)
			cc.mon.SetConfig(godebug.SVar(it))

			return nil
		}
	}

	create_Status := func() func(*cli.Context) error {
		nth := 0
		return func(ctx *cli.Context) error {
			Verbose := ctx.Bool("verbose")
			Periodic := ctx.String("periodic")
			h, _ := GetSize()

			showStatus := func() {
				nth++
				st, _ := cc.mon.GetStatusOfItemVerbose(Verbose)
				// cc.mon.SortByNameStatus(st)
				// fmt.Printf("After 2 : %s\n", lib.SVarI(st))
				fmt.Printf("%s", strings.Repeat("\n", int(h)))
				fmt.Printf("%5d %-30s %-5s %-30s\n", nth%10000, "Name", "Stat.", "Data")
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

	// xyzzy - live monitor version of this - listen for messages
	create_LiveMonitor := func() func(*cli.Context) error {
		nth := 0

		return func(ctx *cli.Context) error {
			Verbose := ctx.Bool("verbose")
			Quiet := ctx.Bool("quiet")
			File := ctx.String("file")
			h := uint(80)
			if !Quiet {
				h, _ = GetSize()
			}

			ms := ListenLib.NewMsCfgType("trx:listen", "")

			ms.RedisConnectHost = qdemolib.ServerGlobal.RedisConnectHost
			ms.RedisConnectPort = qdemolib.ServerGlobal.RedisConnectPort
			ms.RedisConnectAuth = qdemolib.ServerGlobal.RedisConnectAuth

			// SEE: https://redis.io/topics/notifications

			// ms.SetEventPattern("__keyevent@0__:expired")
			ms.SetEventPattern("__keyevent@0__:expire*")

			ms.ConnectToRedis() // Create the redis connection pool, alternative is ms.SetRedisPool(pool) // ms . SetRedisPool(pool *pool.Pool)
			ms.SetRedisConnectInfo(qdemolib.ServerGlobal.RedisConnectHost, qdemolib.ServerGlobal.RedisConnectPort, qdemolib.ServerGlobal.RedisConnectAuth)
			ms.SetupListen()

			showStatus := func(dm map[string]interface{}) {
				// fmt.Printf("dm=%+v\n", dm)

				runIt := false

				cmd_r, ok0 := dm["cmd"]
				cmd, ok1 := cmd_r.(string)
				itemKey_r, ok2 := dm["val"]

				if ok0 && ok1 && ok2 && cmd == "expired" {

					itemKey, ok3 := itemKey_r.(string)

					if ok3 {
						runIt = cc.mon.IsMonitoredItem(itemKey)
					}

				} // check for this having a key name passed in.

				if ok0 && ok1 && cmd != "expired" { // cmd==timeout-call || cmd==at-top
					runIt = true
					// fmt.Printf("dm=%+v\n", dm)
				}

				if runIt {
					st, hasChanged := cc.mon.GetStatusOfItemVerbose(Verbose)
					fmt.Printf("st=%s hasChanged=%v, %s\n", godebug.SVarI(st), hasChanged, godebug.LF())
					if hasChanged {
						if db9 {
							fmt.Printf("For push to Socket.IO: st=%s\n", godebug.SVarI(st))
						}
						if File != "" {
							// File := ctx.String("file")
							ioutil.WriteFile(File, []byte(godebug.SVarI(st)), 0640)
						}

						if !Quiet {
							nth++
							// cc.mon.SortByNameStatus(st)
							// fmt.Printf("After 2 : %s\n", lib.SVarI(st))
							fmt.Printf("%s", strings.Repeat("\n", int(h)))
							fmt.Printf("%5d %-35s %2s %-40s\n", nth%10000, "Name", "St", "Data")
							fmt.Printf("%5s %-35s %2s %-40s\n", "-----", "-----------------------------------", "--", "-----------------------------------")
							for ii, vv := range st {
								vvName := vv.Name
								if len(vvName) > 35 {
									vvName = vvName[0:35]
								}
								if vv.Status == "up" {
									fmt.Printf("%4d: %-35s %s%2s%s %-40s\n", ii, vvName, MiscLib.ColorGreen, vv.Status, MiscLib.ColorReset, vv.Data)
								} else {
									fmt.Printf("%4d: %-33s %s%4s %-40s%s\n", ii, vvName, MiscLib.ColorRed, "down", vv.LongName, MiscLib.ColorReset)
								}
							}
						}
					}
				} // check for key being a monitored item

			}

			var wg sync.WaitGroup

			ms.ListenForServer(showStatus, &wg)

			wg.Wait() // wait forever - server runs in loop. -- On "exit" message it will

			return nil
		}
	}

	/*
	   xyzzyAddCRUD - CRUD on monitored items.
	   func (mon *MonIt) AddNewItem(itemName string, ttl uint64) { // xyzzy - additional params
	   	/Users/corwin/go/src/github.com/pschlump/mon-alive/lib/monAlive.go:511
	   func (mon *MonIt) RemoveItem(itemName string) {
	   func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {
	*/

	create_Trace := func() func(*cli.Context) error {
		return func(ctx *cli.Context) error {
			TrxId := ctx.String("trx-id")
			tfn := ctx.String("tfn")

			RedisHost, RedisPort, RedisAuth := qdemolib.GetRedisConnectInfo()

			ms := MicroServiceLib.NewMsCfgType("trx:listen", "")

			ms.RedisConnectHost = RedisHost
			ms.RedisConnectPort = RedisPort
			ms.RedisConnectAuth = RedisAuth

			ms.ConnectToRedis()                                     // Create the redis connection pool, alternative is ms.SetRedisPool(pool) // ms . SetRedisPool(pool *pool.Pool)
			ms.SetRedisConnectInfo(RedisHost, RedisPort, RedisAuth) // setup the dedicated listener
			ms.SetupListenServer()

			cc.ms = ms

			funcMap := template.FuncMap{
				"json":       lib.SVarI,      // Convert data to JSON format to put into JS variable
				"sqlEncode":  sqlEncode,      // Encode data for use in SQL with ' converted to ''
				"jsEsc":      jsEsc,          // Escape strings for use in JS - with ' converted to \'
				"jsEscDbl":   jsEscDbl,       // Escape strings for use in JS - with " converted to \"
				"rptStr":     strings.Repeat, //
				"padLeft":    padLeft,        //
				"padRight":   padRight,       //
				"toFile":     toFile,         // Print current pipe to file, return ""
				"teeTooFile": teeToFile,      // Print current pipe to file, pass along string
			}

			compiledTemplate, err := template.New("file-template").Funcs(funcMap).ParseFiles(tfn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Template parse error Error: %s\n", err)
				return err
			}

			definedTmpl := compiledTemplate.DefinedTemplates()

			fx := func(dm map[string]interface{}) {
				// fmt.Printf("fx called, data=%s\n", godebug.SVarI(dm))
				cmd := ""
				if cmd_x, ok := dm["cmd"]; ok {
					if cmd_s, ok := cmd_x.(string); ok {
						cmd = cmd_s
					}
				}
				if cmd == "at-top" {
					return
				}
				if cmd == "timeout-call" {
					return
				}
				/*
				   fx called, data={
				   	"ClientTrxId": "9bd8cdd3-8cbb-452e-4f9e-5711b29cb566",
				   	"Path": "/uri-start",
				   	"Scheme": "rps",
				   	"To": "rps://tracer/uri-start",
				   	"maxKey": 121151
				   }
				*/
				clientTrxId := ""
				if ct_x, ok := dm["ClientTrxId"]; ok {
					if ct, ok := ct_x.(string); ok {
						clientTrxId = ct
					}
				}
				if clientTrxId != "" {
					// fmt.Printf("TrxId = [%s], clientTrxId = [%s], AT: %s\n", TrxId, clientTrxId, godebug.LF())
					if TrxId == "" || TrxId == clientTrxId {
						// fmt.Printf("AT: %s\n", godebug.LF())
						maxKey := int64(0) // maxKey := int64(dm["maxKey"].(float64))
						if maxKey_x, ok := dm["maxKey"]; ok {
							if ff, ok := maxKey_x.(float64); ok {
								maxKey = int64(ff)
							}
						}
						op := "" // op := dm["Path"].(string)
						if op_x, ok := dm["Path"]; ok {
							if tt, ok := op_x.(string); ok {
								op = tt
							}
						}
						// fmt.Printf("AT: %s\n", godebug.LF())
						if op == "/uri-end" {
							// fmt.Printf("AT: %s\n", godebug.LF())
							s, k, ok := GetOutput(cc.conn, maxKey)
							if db8 {
								fmt.Printf("s=%s k=%s, ok=%v\n", s, k, ok)
							}

							// var message tr.Trx
							// type Trx struct {
							var message TrxExtended
							message.ColorRed = MiscLib.ColorRed
							message.ColorYellow = MiscLib.ColorYellow
							message.ColorGreen = MiscLib.ColorGreen
							message.ColorCyan = MiscLib.ColorCyan
							message.ColorReset = MiscLib.ColorReset
							h, w := GetSize()
							message.ScreenHeight = int(h)
							message.ScreenWidth = int(w)

							err := json.Unmarshal([]byte(s), &message)
							if err != nil {
								fmt.Printf("%sError on redis/unmarshal - (trx:%06d)/(%s): Error:%s, %s%s\n", MiscLib.ColorRed, maxKey, s, err, godebug.LF(), MiscLib.ColorReset)
							}

							if db7 {
								fmt.Printf("parsed message: %s\n", godebug.SVarI(message))
							}

							// ========================================================================== ==========================================================================
							// Use template to render message to output format.
							// ========================================================================== ==========================================================================
							// xyzzy TODO: 4. Other data (TabServer2)
							// xyzzy TODO: 7. Returned Data to User - Response Body shown
							if strings.Index(definedTmpl, "render") >= 0 {
								err = compiledTemplate.ExecuteTemplate(os.Stdout, "render", message)
								if err != nil {
									fmt.Fprintf(os.Stderr, "Error on rendering temlate, %s\n", err)
								}
							}

						} else {
							if db8 {
								fmt.Printf("op=%v\n", op)
							}
						}
					}
				}
			}

			var wg sync.WaitGroup

			ms.ListenForServer(fx, &wg)

			wg.Wait() // wait forever - server runs in loop. -- On "exit" message it will

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
			Name:   "live-monitor",
			Usage:  "Push Notification: Show the up/down status of monitored processes",
			Action: create_LiveMonitor(),
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.BoolFlag{
					Name:  "quiet, q",
					Usage: "No terminal output - just write to file.",
				},
				cli.StringFlag{
					Name:  "file, f",
					Usage: "Dump output to a file.",
				},
			},
		},
		{
			Name:   "add-item",
			Usage:  "Add new monitoed item",
			Action: create_AddItem(), //  xyzzyAddCRUD - CRUD on monitored items.
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "key, k",
					Usage: "Primary Name of the service.",
				},
				cli.StringFlag{
					Name:  "name, n",
					Usage: "Full name of the service for error messages.",
				},
				cli.StringFlag{
					Name:  "ttl, t",
					Usage: "Timeout before assume that it is down. Default 120 sec.",
				},
				cli.BoolFlag{
					Name:  "ping, P",
					Usage: "Requries a ping to see if really down. (-u/--url must be set)",
				},
				cli.StringFlag{
					Name:  "url, u",
					Usage: "URL to ping to verify up/down status.",
				},
			},
		},
		{
			Name:   "rm-item",
			Usage:  "remove monitored item",
			Action: create_RmItem(),
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "key, k",
					Usage: "Primary Name of the service.",
				},
			},
		},
		{
			Name:   "upd-item",
			Usage:  "update monitored item",
			Action: create_UpdItem(),
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "key, k",
					Usage: "Primary Name of the service.",
				},
				cli.StringFlag{
					Name:  "name, n",
					Usage: "Full name of the service for error messages.",
				},
				cli.StringFlag{
					Name:  "ttl, t",
					Usage: "Timeout before assume that it is down. Default 120 sec.",
				},
				cli.StringFlag{
					Name:  "ping, P",
					Usage: "Requries a ping to see if really down. (-u/--url must be set)",
				},
				cli.StringFlag{
					Name:  "url, u",
					Usage: "URL to ping to verify up/down status.",
				},
			},
		},
		{
			Name:   "enable-item",
			Usage:  "Change existing item to enabled - turn on monetering",
			Action: create_LiveMonitor(), //  xyzzyAddCRUD - CRUD on monitored items.
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "key, k",
					Usage: "Primary Name of the service.",
				},
			},
		},
		{
			Name:   "disable-item",
			Usage:  "Change existing item to disabled - turn OFF monetering",
			Action: create_LiveMonitor(), //  xyzzyAddCRUD - CRUD on monitored items.
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
				cli.StringFlag{
					Name:  "key, k",
					Usage: "Primary Name of the service.",
				},
			},
		},
		{
			Name:   "list-available-item",
			Usage:  "list of available items reporting that can be monitored",
			Action: create_LiveMonitor(), //  xyzzyAddCRUD - CRUD on monitored items.
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "Verbose output when status is displayed.",
				},
			},
		},
		{
			Name:   "trace",
			Usage:  "Trace calls to the server",
			Action: create_Trace(),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "trx-id, T",
					Usage: "Trace a specific session.",
				},
				cli.StringFlag{
					Name:  "tfn, t",
					Value: "./trace-txt.tmpl",
					Usage: "Template file name.",
				},
			},
		},
		// xyzzy - list all trx-id's available to trace ( and user / login status ) - in the last 1/2 hr		T
		// xyzzy - walk backward on a trx-id, given list backward - forward, current							b, f, c
		// xyzzy - list non-users (anonomous / not logged in folks)												a
		// xyzzy - list users logged in																			u
		// xyzzy - watch all queries																			*
		// xyzzy - watch all requests																			+
		// xyzzy - get load levels																				?
		// xyzzy - start new service																			^	Ms
		// xyzzy - stop service																					!	Md
		// xyzzy - set notification destination																	M	Mn
		// xyzzy - set notification conditions																	M	Mc
		// xyzzy - set actions and conditions to take actions (start/stop microserice, servers, etc)			M	Ma
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

//------------------------------------------------------------------------------------------------
// blog on this
// StringSliceDesc attaches the methods of Interface to []string, sorting in increasing order.
type StringSliceDesc []string

func (p StringSliceDesc) Len() int           { return len(p) }
func (p StringSliceDesc) Less(i, j int) bool { return p[i] > p[j] }
func (p StringSliceDesc) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p StringSliceDesc) Sort() { sort.Sort(p) }

// Strings sorts a slice of strings in increasing order.
func SortStringsDesc(a []string) { sort.Sort(StringSliceDesc(a)) }

//------------------------------------------------------------------------------------------------
func GetOutput(conn *redis.Client, theKey int64) (s, k string, ok bool) {
	key := fmt.Sprintf(`trx:%06d`, theKey)
	// s, err = redis.String(tr.RedisDo("GET", key))
	s, err := conn.Cmd("GET", key).Str()
	if err == nil {
		rv := GetKeysTrx(conn, "trx:*")
		k = godebug.SVar(rv)
		if k == "" {
			k = "[]"
		}
		ok = true
		return
	}
	return fmt.Sprintf(`{"Error":"%s"}`, err), "[]", false
}

//------------------------------------------------------------------------------------------------
func GetKeysTrx(conn *redis.Client, theKey string) []string {
	// kk, err := redis.Strings(tr.RedisDo("KEYS", theKey))
	kk, err := conn.Cmd("KEYS", theKey).List()
	if err != nil {
		return []string{}
	}
	kks := make([]string, len(kk), len(kk))
	for i, kv := range kk {
		x, _ := strconv.Atoi(kv[4:])
		kks[i] = fmt.Sprintf("%d", x)
	}
	SortStringsDesc(kks)
	if len(kks) > 100 {
		return kks[0:100]
	} else {
		return kks
	}
}

//------------------------------------------------------------------------------------------------

func sqlEncode(s string) (rv string) {
	rv = strings.Replace(s, "'", "''", -1)
	return
}

func jsEsc(s string) (rv string) {
	fmt.Printf("s=%s\n", s)
	rv = strings.Replace(s, "'", `\'`, -1)
	return
}
func jsEscDbl(s string) (rv string) {
	rv = strings.Replace(s, `"`, `\"`, -1)
	return
}

func padLeft(width int, s string) string {
	format := fmt.Sprintf("%%%ds", width)
	return fmt.Sprintf(format, s)
}

func padRight(width int, s string) string {
	format := fmt.Sprintf("%%-%ds", width)
	return fmt.Sprintf(format, s)
}

func toFile(fn string, s string) string {
	ioutil.WriteFile(fn, []byte(s+"\n"), 0600)
	return ""
}

func teeToFile(fn string, s string) string {
	ioutil.WriteFile(fn, []byte(s), 0600)
	return s
}

const db7 = false
const db8 = false
const db9 = false
