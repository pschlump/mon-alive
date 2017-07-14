package MonAliveLib

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/godebug" //	Modifed from: "encoding/json"
	"github.com/pschlump/json"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

// TODO ; 1. Get list of up/down systems based on Group search -- ONLY check systesm that match the group
// TODO ; 1. Get list groups

// TODO ; sort the return set of items up/down - by name
// TODO ; Add in notification destination and action for down items

/*
redis-cli -h 192.168.0.139
192.168.0.139:6379> auth lLJSmkccYJiVEwskr1RM4MWIaBM
OK
192.168.0.139:6379> psubscribe '__key*__:expire*'
-- Just pick up exipre
192.168.0.139:6379> psubscribe '__key*__:monitor:* expire'			-- ?? just monitor expires(?)
*/

type ConfigItem struct {
	Name         string                 `json:"Name"`         // Extended name for this item
	TTL          uint64                 `json:"TTL"`          // How long before should have received PING on item
	RequiresPing bool                   `json:"RequiresPing"` // To determine if it is alive requires a "ping" -- Maybe keep track of delta-t on last ping and only so often
	PingUrl      string                 `json:"PingUrl"`      // URL to do "get" on to ping item -- http://localhost:16040/api/status for example
	Group        []string               `json:"Group"`        // Set of groups that this belongs to "host":"virtual-host", "host":"goftl-server"
	CdTo         string                 `json:"CdTo"`         // Direcotry to chagne to before run
	CmdToRun     string                 `json:"CmdToRun"`     // Command to run when notification is needed.
	seen         bool                   `json:"-"`            // marker - what has been seen, so can find not seen and report as down.
	Extra        map[string]interface{} // Other Config Items...
}

type ConfigMonitor struct {
	Item     map[string]ConfigItem //
	MinTTL   int                   // Defaults to 30 seconds - shortest time that can be set in seconds
	TrxIdTTL int                   // 0 for forever, else length in seconds for Trx:Id tracing to last on an Trx:Id
}

type MonIt struct {
	GetConn  func() (conn *redis.Client)
	FreeConn func(conn *redis.Client)
}

func NewMonIt(GetConn func() (conn *redis.Client), FreeConn func(conn *redis.Client)) (rv *MonIt) {
	getConn := GetConn
	freeConn := FreeConn
	if freeConn == nil {
		freeConn = func(conn *redis.Client) {}
	}
	rv = &MonIt{
		GetConn:  getConn,
		FreeConn: freeConn,
	}
	return
}

func (mon *MonIt) UpdateConfig() (rv ConfigMonitor) {
	rv.MinTTL = 30
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	s, err := conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		os.Exit(1)
		return
	}
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}

	nh := make(map[string]bool)
	for ii, vv := range rv.Item {
		if _, ok := nh[vv.Name]; ok {
			fmt.Printf("At %s in condfig data, duplicate name will result in 'down' state even when item is running (%s)", ii, vv.Name)
		} else {
			nh[vv.Name] = true
		}
	}

	return
}

func (mon *MonIt) SendIAmAlive(itemName string, myStatus map[string]interface{}) {
	u := mon.UpdateConfig()
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	cfgForItem, ok := u.Item[itemName]
	if !ok {
		conn.Cmd("SADD", "monitor:potentialItem", itemName) // add to set of "could-be-monitored-items"
		return                                              // not a monitored item at this time - just return
	}
	ttl := cfgForItem.TTL
	conn.Cmd("SREM", "monitor:potentialItem", itemName) // Actually monitoring this item
	conn.Cmd("SADD", "monitor:IAmAlive", itemName)
	myStatus["status"] = "ok"
	ms := lib.SVar(myStatus)
	//onn.Cmd("SET", "monitor::"+itemName, ms)
	//conn.Cmd("EXPIRE", "monitor::"+itemName, ttl)
	conn.Cmd("SETEX", "monitor::"+itemName, ms, ttl)
}

func (mon *MonIt) SetupStatus(listenAt string, fxStatus func() string, fxTest func() bool) {

	status := func(res http.ResponseWriter, req *http.Request) {
		s := fxStatus()
		res.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(res, s)
	}

	test := func(res http.ResponseWriter, req *http.Request) {
		ok := fxTest()
		res.Header().Set("Content-Type", "application/json")
		if ok {
			fmt.Fprintf(res, `{"status":"succcess"}`)
		} else {
			fmt.Fprintf(res, `{"status":"failed"}`)
		}
	}

	go func() {
		http.HandleFunc("/api/status", status)
		http.HandleFunc("/api/test", test)
		log.Fatal(http.ListenAndServe(listenAt, nil))
	}()

}

/*


// -------------------------------------------------------------------------------------------------
func respHandlerStatus(res http.ResponseWriter, req *http.Request) {
	q := req.RequestURI

	var rv string
	res.Header().Set("Content-Type", "application/json")
	rv = fmt.Sprintf(`{"status":"success","name":"go-server version 1.0.0","URI":%q,"req":%s, "response_header":%s}`, q, SVarI(req), SVarI(res.Header()))

	io.WriteString(res, rv)
}

// -------------------------------------------------------------------------------------------------
*/

// shutdown op -- intentionally shutdown - means no notificaiton
func (mon *MonIt) SendIAmShutdown(itemName string) {
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	// fmt.Printf("AT: %s\n", godebug.LF())
	conn.Cmd("SADD", "monitor:potentialItem", itemName) // Actually monitoring this item
	// fmt.Printf("AT: %s\n", godebug.LF())
	conn.Cmd("SREM", "monitor:IAmAlive", itemName)
	// fmt.Printf("AT: %s\n", godebug.LF())
	err := conn.Cmd("DEL", "monitor::"+itemName).Err
	// fmt.Printf("AT: %s, Del on monitor::%s -- err=%s\n", godebug.LF(), itemName, err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove monitor::%s in redis - that is not good, %s\n", itemName, godebug.LF())
		return
	}
}

// shutdown op -- I know that I have failed - I should be up but I have AbEnded
func (mon *MonIt) SendIFailed(itemName string) {
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	// fmt.Printf("AT: %s\n", godebug.LF())
	err := conn.Cmd("DEL", "monitor::"+itemName).Err
	// fmt.Printf("AT: %s, Del on monitor::%s -- err=%s\n", godebug.LF(), itemName, err)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove monitor::%s in redis - that is not good, %s\n", itemName, godebug.LF())
		return
	}
}

// Create a timed I Am Alive message
func (mon *MonIt) SendPeriodicIAmAlive(itemName string) {

	u := mon.UpdateConfig()
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	minTtl := u.MinTTL
	cfgForItem, ok := u.Item[itemName]
	if !ok {
		conn.Cmd("SADD", "monitor:potentialItem", itemName) // add to set of "could-be-monitored-items"
		cfgForItem = ConfigItem{
			TTL:  120,
			Name: itemName,
		}
		fmt.Printf("Using Default Config, for %s\n", itemName)
	}
	ttl := cfgForItem.TTL

	// timer with period 3/4 TTL - and send ping to moitor it -- create a go-routine with TTL and in a loop on channel
	calcTtl := int(float32(ttl) * 3 / 4)
	if calcTtl < minTtl {
		fmt.Printf("Error: calculated TTL is too short -must- be %d or larger in seconds, setting to %d\n", minTtl, minTtl)
		calcTtl = minTtl
	}

	myStatus := make(map[string]interface{})

	go func() {
		// fmt.Printf("AT: %s\n", godebug.LF())
		itemNameCpy := itemName
		ticker := time.NewTicker(time.Duration(calcTtl) * time.Second)
		for {
			if db4 {
				fmt.Printf("Duration for Ticker %d, AT: %s\n", calcTtl, godebug.LF())
			}
			select {
			case <-ticker.C:
				if db1 {
					fmt.Printf("periodic IAmAlive(%s) ticker...\n", itemNameCpy)
				}
				mon.SendIAmAlive(itemNameCpy, myStatus)
			}
		}
	}()

}

// GetNotifyItem returns the set of items that is NOT running that should be running
// Eventually - check via circuit checs for items that require ping
// URL: /api/mon/get-notify-item
func (mon *MonIt) GetNotifyItem() (rv []string) {
	// get all items - get notify items - do DIFF and see if not being pinged
	// get the set of items that are being monitored -- monitor:IAmAlive
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	it, err := conn.Cmd("SMEMBERS", "monitor:IAmAlive").List()
	if err != nil {
		fmt.Printf("Error getting 'SMEMBERS', 'monitor:IAmAlive', err=%s\n", err)
		return
	}
	// Iterate over set and check to see what keys are missing
	for ii, vv := range it {
		item, err := conn.Cmd("GET", "monitor::"+vv).Str()
		// fmt.Printf("GET monitor::%s, err=%s item=%s, %s\n", vv, err, item, godebug.LF())
		if err != nil {
			// rv = append(rv, fmt.Sprintf("Item: %s - error %s\n", vv, err))
			rv = append(rv, vv)
		} else if item == "" {
			fmt.Fprintf(os.Stderr, "Item: %s - found, no data\n", vv)
		} else {
			if db3 {
				fmt.Printf("Found %s at %d in set - it's ok, %s\n", vv, ii, godebug.LF())
			}
			// do nothing - it's ok
		}
	}
	return
}

func DoGet(url string) (string, bool) {
	client := http.Client{nil, nil, nil, 0}
	r1, e0 := client.Get(url)
	if e0 != nil {
		fmt.Printf("\tError: %v, %s\n", e0, godebug.LF())
		return "Error", false
	}
	rv, e1 := ioutil.ReadAll(r1.Body)
	if e1 != nil {
		fmt.Printf("\tError: %v, %s\n", e1, godebug.LF())
		return "Error", false
	}
	r1.Body.Close()
	if string(rv[0:6]) == ")]}',\n" {
		rv = rv[6:]
	}

	return string(rv), true
}

//func SendIAmAlive(item string, note string) (ok bool, rv string) {
//	client := http.Client{nil, nil, nil, 0}
//	s := ""
//	e := false
//	if u, ok := GlobalCfg["I_Am_Alive_URL"]; ok {
//		s, e = DoGet(&client, u)
//	} else {
//		// EmailSent-Q1
//		s, e = DoGet(&client, GlobalCfg["monitor_url"]+"/api/ping_i_am_alive?item="+item+"&note=Ok"+note)
//	}
//	rv = s
//	ok = e
//	return
//}

type Countries []ItemStatus

func (slice Countries) Len() int {
	return len(slice)
}

func (slice Countries) Less(i, j int) bool {
	return slice[i].Name < slice[j].Name
}

func (slice Countries) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// cc.mon.SortByNameStatus(&st)
func (mon *MonIt) SortByNameStatus(st Countries) {
	// fmt.Printf("Before Sort: %s\n", godebug.SVarI(st))
	sort.Sort(st)
	// fmt.Printf("After 1 Sort: %s\n", godebug.SVarI(st))
}

type ItemStatus struct {
	Name     string
	Status   string
	Data     string
	LongName string
}

// GetStatusOfItemVerbose returns the set of items that is NOT running that should be running
// Eventually - check via circuit checs for items that require ping
// URL: /api/mon/get-notify-item
func (mon *MonIt) GetStatusOfItemVerbose(extra bool) (rv []ItemStatus) {
	// get all items - get notify items - do DIFF and see if not being pinged
	// get the set of items that are being monitored -- monitor:IAmAlive
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	it, err := conn.Cmd("SMEMBERS", "monitor:IAmAlive").List()
	if err != nil {
		fmt.Printf("Error getting 'SMEMBERS', 'monitor:IAmAlive', err=%s\n", err)
		return
	}
	u := mon.UpdateConfig()
	// Iterate over set and check to see what keys are missing
	for ii, vv := range it {
		up := true
		item, err := conn.Cmd("GET", "monitor::"+vv).Str()
		// fmt.Printf("GET monitor::%s, err=%s item=%s, %s\n", vv, err, item, godebug.LF())
		Item, ok := u.Item[vv]
		if ok {
			Item.seen = true
			longName := Item.Name
			if err != nil {
				// rv = append(rv, fmt.Sprintf("Item: %s - error %s\n", vv, err))
				rv = append(rv, ItemStatus{Name: vv, Status: "down", LongName: longName})
				up = false
			} else if item == "" {
				fmt.Fprintf(os.Stderr, "Item: %s - found, no data\n", vv)
				rv = append(rv, ItemStatus{Name: vv, Status: "up", Data: "", LongName: longName})
			} else {
				if db3 {
					fmt.Printf("Found %s at %d in set - it's ok, %s\n", vv, ii, godebug.LF())
				}
				rv = append(rv, ItemStatus{Name: vv, Status: "up", Data: item, LongName: longName})
			}
			if !up {
				Item := u.Item[vv]
				if Item.RequiresPing {
					fmt.Printf("Ping required to verify it is down, %s\n", Item.PingUrl)
				}
			}
			if !up {
				if Item.RequiresPing {
					_, ok := DoGet(Item.PingUrl)
					if ok {
						Item.seen = true
						rv[len(rv)-1] = ItemStatus{Name: vv, Status: "up", Data: item, LongName: "Ping check " + Item.PingUrl + " checks ok"}
					}
				}
			}
			u.Item[vv] = Item
		} else {
			if extra {
				if err != nil {
					rv = append(rv, ItemStatus{Name: vv, Status: "down", Data: " (u) ", LongName: vv + " (( Unexpected - Not in config ))"})
				} else {
					rv = append(rv, ItemStatus{Name: vv, Status: "up", Data: " (u) ", LongName: vv + " (( Unexpected - Not in config ))"})
				}
			}
		}
	}
	for ii, vv := range u.Item {
		if !vv.seen {
			rv = append(rv, ItemStatus{Name: ii, Status: "down", Data: "", LongName: vv.Name + " ((Not Seen))"})
			if vv.RequiresPing {
				_, ok := DoGet(vv.PingUrl)
				if ok {
					rv[len(rv)-1] = ItemStatus{Name: ii, Status: "up", Data: "Ping:" + vv.PingUrl, LongName: "Ping check " + vv.PingUrl + " checks ok."}
				}
			}
		}
	}
	return
}

func StatusOf(itemName string, allStat []ItemStatus) (rv string) {
	for _, vv := range allStat {
		if vv.Name == itemName {
			return vv.Status
		}
	}
	return "up" // keep positive in uncertain times
}

//type ItemStatus struct {
//	Up       string
//	Name     string
//	LongName string
//}

// GetItemStatus - up/down - all items monitored.
// URL: /api/mon/item-status
func (mon *MonIt) GetItemStatus() (rv []ItemStatus) {
	u := mon.UpdateConfig()
	dn := mon.GetNotifyItem()
	for ii, vv := range u.Item {
		trv := ItemStatus{Status: "up", Name: ii, LongName: vv.Name}
		if lib.InArray(ii, dn) {
			trv.Status = "down"
		}
		rv = append(rv, trv)
	}
	return
}

// return the set of all the named items that are being monitored
// URL: /api/mon/get-all-item
func (mon *MonIt) GetAllItem() (rv []string) {
	u := mon.UpdateConfig()
	for ii := range u.Item {
		rv = append(rv, ii)
	}
	return
}

// add an item to the set of items that is monitored
// URL: /api/mon/add-new-item?itemName= ttl= ...
func (mon *MonIt) AddNewItem(itemName string, ttl uint64) { // xyzzy - additional params

	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	// fmt.Printf("AT: %s\n", godebug.LF())
	s, err := conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	// fmt.Printf("AT: %s\n", godebug.LF())
	var rv ConfigMonitor
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	// fmt.Printf("AT: %s\n", godebug.LF())
	if vv, ok := rv.Item[itemName]; ok {
		vv.TTL = ttl
	} else {
		rv.Item[itemName] = ConfigItem{
			Name: itemName,
			TTL:  ttl,
			// RequiresPing bool                   // To determine if it is alive requires a "ping" -- Maybe keep track of delta-t on last ping and only so often
			// PingUrl      string                 // URL to do "get" on to ping item -- /api/status for example
			// Group        []string               // Set of groups that this belongs to "host":"virtual-host", "host":"goftl-server"
			// Extra        map[string]interface{} // Other Config Items...
		}
	}

	// fmt.Printf("AT: %s\n", godebug.LF())
	s = lib.SVar(rv)
	if db2 {
		s = lib.SVarI(rv)
	}
	// fmt.Printf("AT: %s, s=>%s<\n", godebug.LF(), lib.SVarI(rv))
	err = conn.Cmd("SET", "monitor:config", s).Err
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to save updated configuration to monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	// fmt.Printf("AT: %s\n", godebug.LF())

}

// URL: /api/mon/rem-item?itemName=
func (mon *MonIt) RemoveItem(itemName string) {

	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	s, err := conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	var rv ConfigMonitor
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	if _, ok := rv.Item[itemName]; ok {
		// fmt.Printf("AT: %s -- remvoing item %s\n", godebug.LF(), itemName)
		delete(rv.Item, itemName)
	}

	s = lib.SVar(rv)
	if db2 {
		s = lib.SVarI(rv)
	}
	err = conn.Cmd("SET", "monitor:config", s).Err
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to save updated configuration to monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}

	err = conn.Cmd("DEL", "monitor::"+itemName).Err
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to remove monitor::%s in redis - that is not good, %s\n", itemName, godebug.LF())
		return
	}

}

// URL: /api/mon/upd-config-item?itemName=, ...
func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {

	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	s, err := conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	var rv ConfigMonitor
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}

	ttl, ok := newConfig["TTL"].(uint64)
	if !ok {
		ttl = 60
	}
	requiresPing, ok := newConfig["RequiresPing"].(bool)
	if !ok {
		requiresPing = false
	}
	pingUrl, ok := newConfig["PingUrl"].(string)
	if !ok {
		requiresPing = false
	}
	group, ok := newConfig["Group"].([]string)
	if !ok {
		group = []string{}
	}

	if vv, ok := rv.Item[itemName]; ok {
		vv.TTL = ttl
	} else {
		rv.Item[itemName] = ConfigItem{
			Name:         itemName,
			TTL:          ttl,
			RequiresPing: requiresPing,
			PingUrl:      pingUrl,
			Group:        group,
		}
	}

	s = lib.SVar(rv)
	if db2 {
		s = lib.SVarI(rv)
	}
	err = conn.Cmd("SET", "monitor:config", s).Err
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to save updated configuration to monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}

}

// set config from file
// URL: /api/mon/reload-config?fn=
func (mon *MonIt) SetConfigFromFile(fn string) {
	s, err := ioutil.ReadFile(fn)
	if err != nil {
		return
	}
	conn := mon.GetConn()
	err = conn.Cmd("SET", "monitor:config", s).Err
	mon.FreeConn(conn)
	if err != nil {
		fmt.Printf("Error: %s seting configuration - File: %s\n", err, fn)
		return
	}
}

// get to set of "could-be-monitored-items"
// URL: /api/mon/list-potential
func (mon *MonIt) GetListOfPotentialItem() (rv []string) {
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	it, err := conn.Cmd("SMEMBERS", "monitor:potentialItem").List()
	if err != nil {
		fmt.Printf("Error getting 'SMEMBERS', 'monitor:IAmAlive', err=%s\n", err)
		return
	}
	rv = it
	return
}

func (mon *MonIt) GetConfig() (s string) {
	var err error
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	s, err = conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Printf("Error: %s getting configuration - may be empty/not-set\n", err)
		return
	}
	return
}

/*
	/api/mon/trx?state=on|off
	trx:state (on|off)
		Turn tracking on/off for this Trx-ID -  if via SIO - then sends message via Pub/Sub to all servers to turn this ID on.  If via /api/mon/trx then
		if via /api/mon/trx - then sends Pub/Sub to tracer - to tell tracter to send message to pass on.
*/
func (mon *MonIt) SendTrxState(state, trxId string) {
	ss := struct {
		TrxId string
		State string
	}{
		TrxId: trxId,
		State: state,
	}
	conn := mon.GetConn()
	defer mon.FreeConn(conn)
	err := conn.Cmd("PUBLISH", "monitor:trx-state", lib.SVar(ss)).Err
	if err != nil {
		fmt.Printf("Error: %s publishing monitor:trx-state\n", err)
	}
}

const db1 = false
const db2 = false
const db3 = false
const db4 = false

/* vim: set noai ts=4 sw=4: */
