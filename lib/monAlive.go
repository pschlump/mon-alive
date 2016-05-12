package MonAliveLib

import (
	"fmt"
	"github.com/pschlump/json" //	Modifed from: "encoding/json"
	"os"
)

type ConfigItem struct {
	Name         string
	TTL          uint64
	RequiresPing bool // To determine if it is alive requires a "ping" -- Maybe keep track of delta-t on last ping and only so often
}

type ConfigMonitor struct {
	Items map[string]ConfigItem
}

func UpdateConfig() (rv ConfigMonitor) {
	// TODO: conn
	s, err := conn.Cmd("GET", "monitor:config").Str()
	err = json.Unmarshal([]byte(s), &rv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse the configuration - that is not good, %s\n", err)
	}
	return
}

func SendIAmAlive(itemName string) {
	u := UpdateConfig()
	s, err := conn.Cmd("GET", "monitor:config").Str()
	cfgForItem, ok := u.Items[itemName]
	if !ok {
		return // not a monitored item
	}
	ttl := cfgForItem.TTL
	conn.Cmd("SADD", "monitor:IAmAlive", itemName)
	conn.Cmd("SET", "monitor:"+itemName, "ok") // change this to item status as param
	conn.Cmd("EXPIRE", "monitor:"+itemName, ttl)
}

// Create a timed I Am Alive message
func SendPeriodicIAmAlive(itemName string) {
	// TODO: timer with period 3/4 TTL - and send ping to moitor it
}

// Return the set of items that is NOT running that should be running
// Eventually - check via circuit checs for items that require ping
func GetNotifyItems() {
	// TODO: -- get all items - get notify items - do DIFF and see if not being pinged
}

// return the set of all the named items that are being monitored
func GetAllItems() (rv []string) {
	u := UpdateConfig()
	for ii := range u.Items {
		rv = append(rv, ii)
	}
	return
}

// add an item to the set of items that is monitored
func AddNewItem(itemName string, ttl uint64) {
	// TODO:
}
