//
// Monitor Middlware Library
//
// Copyright (C) Philip Schlump, 2014-2016
//
package MonAliveLib

import (
	"os"
	"testing"

	"github.com/pschlump/mon-alive/qdemolib"
	"github.com/pschlump/radix.v2/redis"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------
func Test_MonAliveLib(t *testing.T) {

	tests := []struct {
		cmd      string
		expect   string
		itemName string
	}{
		{
			cmd:      "SendIAmAlive",
			expect:   `?TODO?`,
			itemName: `bob`,
		},
	}

	qdemolib.SetupRedisForTest("../global_cfg.json")

	monClient, isCon := qdemolib.GetRedisClient()
	if !isCon {
		t.Fatalf("Error connecting to Redis - fatal\n")
		os.Exit(1)
	}
	mon := NewMonIt(func() *redis.Client { return monClient }, func(conn *redis.Client) {})

	conn, _ := qdemolib.GetRedisClient()

	for ii, test := range tests {

		switch test.cmd {
		case "SendIAmAlive":
			myStatus := make(map[string]interface{})
			mon.SendIAmAlive(test.itemName, myStatus)
			err := conn.Cmd("GET", "monitor:bob").Err
			if err != nil {
				t.Errorf("Test %2d, Expected to find a key - did not\n", ii)
			}
		default:
			t.Errorf("Test %2d,  invalid test case, %s\n", ii, test.cmd)
		}
	}

}

/*
func (mon *MonIt) SendIAmAlive(itemName string, myStatus map[string]interface{}) {
func (mon *MonIt) SendIAmShutdown(itemName string) {
func (mon *MonIt) GetNotifyItem() (rv []string) {
func (mon *MonIt) GetItemStatus() (rv []ItemStatus) {
func (mon *MonIt) GetAllItem() (rv []string) {
func (mon *MonIt) AddNewItem(itemName string, ttl uint64) {
func (mon *MonIt) RemoveItem(itemName string) {
func (mon *MonIt) ChangeConfigOnItem(itemName string, newConfig map[string]interface{}) {
func (mon *MonIt) SetConfigFromFile(fn string) {
func (mon *MonIt) GetListOfPotentialItem() (rv []string) {
	! TODO !
* func (mon *MonIt) SendPeriodicIAmAlive(itemName string) {

		//if b != test.expectedBody {
		//	t.Errorf("Error %2d, reject error got: %s, expected %s\n", ii, b, test.expectedBody)
		//}

	// mon.SendPeriodicIAmAlive("Go-FTL")
	_ = mon
*/

/* vim: set noai ts=4 sw=4: */
