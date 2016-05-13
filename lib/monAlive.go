package MonAliveLib

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/pschlump/godebug" //	Modifed from: "encoding/json"
	"github.com/pschlump/json"
	"github.com/pschlump/radix.v2/redis" // Modified pool to have NewAuth for authorized connections
)

// done ; 0. Make this into middleware in Go-FTL
// done ; 0. An URL that will send an "i-am-alive" to a item - based on get/post on URL		/api/mon/i-am-alive?itemName
// done ; 0. An URL that will kill/die an "i-am-alive" to a item - based on get/post on URL -- I AM Shutting Down Now - Dead /api/mon/shutdown-now?itemName

// TODO ; 1. Get list of up/down systems based on Group search -- ONLY check systesm that match the group
// TODO ; 1. Get list groups

// done ; 3. other methods for pinging like pub/sub in redis or query to database - maybee CLI for ping
// TODO ; 4. push notification? how - to chat bot?
// TODO ; 5. push notification? how - to log - where it can be picked up and pushed to Twillow? / to SMS? to Email?
// TODO ; 6. create daemon - to SIO push the monitored content out
// TODO ; 7. Periodic "get" and check operations - to poll - websites for alive - working
// TODO ; 7. Periodic run script and get status
// TODO ; 7. OnTime run script and get status -- check config on system -- Use SSH to connect to system and check config

// Note:
//	https://prometheus.io/ -- read consolidate logs -- notification

type ConfigItem struct {
	Name         string                 // Extended name for this item
	TTL          uint64                 // How long before should have received PING on item
	RequiresPing bool                   // To determine if it is alive requires a "ping" -- Maybe keep track of delta-t on last ping and only so often
	PingUrl      string                 // URL to do "get" on to ping item -- /api/status for example
	Group        []string               // Set of groups that this belongs to "host":"virtual-host", "host":"goftl-server"
	Extra        map[string]interface{} // Other Config Items...
}

type ConfigMonitor struct {
	Item   map[string]ConfigItem //
	MinTTL int                   // Defaults to 30 seconds
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
	s, err := conn.Cmd("GET", "monitor:config").Str()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to find the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
	}
	mon.FreeConn(conn)
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration for MonAliveLib - monitor:config in redis - that is not good, %s, %s\n", err, godebug.LF())
		return
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
	conn.Cmd("SET", "monitor::"+itemName, ms)
	conn.Cmd("EXPIRE", "monitor::"+itemName, ttl)
}

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
		return                                              // not a monitored item at this time - just return
	}
	ttl := cfgForItem.TTL

	// timer with period 3/4 TTL - and send ping to moitor it -- create a go-routine with TTL and in a loop on channel
	calcTtl := int(float32(ttl) * 3 / 4)
	if calcTtl < minTtl {
		fmt.Printf("Error: calculated TTL is too short -must- be %d or larger in seconds\n", minTtl)
		return
	}

	myStatus := make(map[string]interface{})

	go func() {
		// fmt.Printf("AT: %s\n", godebug.LF())
		ticker := time.NewTicker(time.Duration(calcTtl) * time.Second)
		for {
			select {
			case <-ticker.C:
				if db1 {
					fmt.Printf("periodic IAmAlive(%s) ticker...\n", itemName)
				}
				mon.SendIAmAlive(itemName, myStatus)
			}
		}
	}()

}

// Return the set of items that is NOT running that should be running
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
			fmt.Sprintf("Item: %s - not founds\n", vv)
		} else {
			if db3 {
				fmt.Printf("Found %s at %d in set - it's ok, %s\n", vv, ii, godebug.LF())
			}
			// do nothing - it's ok
		}
	}
	return
}

func StatusOf(itemName string, allStat []ItemStatus) (rv string) {
	for _, vv := range allStat {
		if vv.Name == itemName {
			return vv.Up
		}
	}
	return "up" // keep positive in uncertain times
}

type ItemStatus struct {
	Up       string
	Name     string
	LongName string
}

// GetItemStatus - up/down - all items monitored.
// URL: /api/mon/item-status
func (mon *MonIt) GetItemStatus() (rv []ItemStatus) {
	u := mon.UpdateConfig()
	dn := mon.GetNotifyItem()
	for ii, vv := range u.Item {
		trv := ItemStatus{Up: "up", Name: ii, LongName: vv.Name}
		if lib.InArray(ii, dn) {
			trv.Up = "down"
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
	// conn.Cmd("SADD", "monitor:potentialItem", itemName) // add to set of "could-be-monitored-items"
	// xyzzy TODO:
	return
}

func (mon *MonIt) GetConfig() (s string) {
	var err error
	conn := mon.GetConn()
	s, err = conn.Cmd("GET", "monitor:config").Str()
	mon.FreeConn(conn)
	if err != nil {
		fmt.Printf("Error: %s getting configuration - may be empty/not-set\n", err)
		return
	}
	return
}

const db1 = false
const db2 = false
const db3 = false
