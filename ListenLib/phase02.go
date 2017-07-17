package ListenLib

//
// Copyright (C) Philip Schlump, 2015-2016.
//

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	newUuid "github.com/pborman/uuid"
	"github.com/pschlump/MiscLib"
	"github.com/pschlump/godebug"
	"github.com/pschlump/radix.v2/pool" // Modified pool to have NewAuth for authorized connections
	"github.com/pschlump/radix.v2/pubsub"
	"github.com/pschlump/radix.v2/redis"
	"github.com/pschlump/uuid"
	"github.com/taskcluster/slugid-go/slugid"
)

// "www.2c-why.com/h2ppp/lib/H2pppCommon"
// "github.com/pschlump/Go-FTL/server/sizlib"

//
// xyzzy - q size, notify on q larger than X or q wait longer than x
// xyzzy - q action - if q larger than X then ... -- start server --
//

//
// Bot - the micro service - lisents, works, replys
// Cli - the command line test for this, start, actions(interpreted)
//	$ Cli/Cli -h 192.168.0.133 -p RedisPort -a RedisPass -c cfgFile.json -i input -o output
//	>>> s Message1 Take-Sec				# send a message to Bot taking Sec of time
//  >>> lr								# List Reply Fx data
//	>>> s Message2 Take-Sec				# send a message to Bot taking Sec of time
//  >>> lr								# List Reply Fx data
//  >>>
//  *** MessageReply: Message1
//  >>>
//  *** MessageReply: Message2
//  >>>
//  >>>
//  >>> q                               # quit
//

type MsMessageParams struct {
	Name  string
	Value string
}

type MsMessageToSend struct {
	Id            string            `json:"Id"`                 // Unique id UUID for this message
	ActionId      string            `json:"ActionId,omitempty"` // Activity Hash ID -- unique to this activity
	GroupId       string            `json:"GroupId,omitempty"`  // Group of id's together ID -- unique to group
	CallId        string            `json:"CallId,omitempty"`   // Unique id for this call - if empty - generated and return
	ReplyTTL      uint64            `json:"ReplyTTL"`           // how long will a reply last if not picked up.
	To            string            `json:"-"`                  // Desitnation work Q - a name -
	ClientTimeout uint64            `json:"-"`                  // How long are you giving client to perform task
	Params        []MsMessageParams `json:"Params"`             // Set of params for client
	IsTimeout     bool              `json:"-"`                  // set on reply
}

type ReplyFxType struct {
	Key        string                                                        // translated ID for reply
	IsTimedOut bool                                                          // Flag indicating timeout occured and fx called with that.
	IsDead     bool                                                          // Flag indicating timeout occured and fx called with that.
	TickCount  int                                                           // Number of time ticks encounted
	TickMax    int                                                           // Max before timeout is set
	Fx         func(data *MsMessageToSend, rx *ReplyFxType, isTimedOut bool) //function that will get called when Key is found
}

// Redis Connect Info ( 2 channels )
type MsCfgType struct {
	ServerId            string                              `json:"Sid"`              //
	Name                string                              `json:"QName"`            //	// Name of the Q to send stuff to //
	ReplyTTL            uint64                              `json:"ReplyTTL"`         // how long will a reply last if not picked up.
	isRedisConnected    bool                                `json:"-"`                // If connect to redis has occured for wakie-wakie calls
	RedisConnectHost    string                              `json:"RedisConnectHost"` // Connection infor for Redis Database
	RedisConnectPort    string                              `json:"RedisConnectPort"` //
	RedisConnectAuth    string                              `json:"RedisConnectAuth"` //
	ReplyListenQ        string                              `json:"ReplyListenQ"`     // Receives Wakie Wakie on Return
	RedisPool           *pool.Pool                          `json:"-"`                // Pooled Redis Client connectioninformation
	Err                 error                               `json:"-"`                // Error Holding Pen
	subClient           *pubsub.SubClient                   `json:"-"`                //
	subChan             chan *pubsub.SubResp                `json:"-"`                //
	timeout             chan bool                           `json:"-"`                //
	ReplyFx             map[string]*ReplyFxType             `json:"-"`                //	Set of call/respond that we are waiting for answers on.
	DebugTimeoutMessage bool                                `json:"-"`                // turn on/off the timeout 1ce a second message
	TickInMilliseconds  int                                 `json:"-"`                // # of miliseconds for 1 tick
	ReplyFunc           func(replyMessage *MsMessageToSend) // function that will get when reply occures
	EventPattern        string                              `json:"EventPattern"` // Pattern for messages to listen to
}

type WorkFuncType func(arb map[string]interface{})

func NewMsCfgType(qName, qReply string) (ms *MsCfgType) {
	var err error
	// ms.ReplyListenQ = "cli-test1-reply"                   // xyzzy-from-config -- template --
	qr := "q-r:%{ServerId%}" // This is the lisened to by client for wakie-wakie on client side
	if qReply != "" {
		qr = qReply
	}
	ms = &MsCfgType{
		ServerId:           UUIDAsStrPacked(),             //
		Err:                err,                           //
		Name:               qName,                         // Name of message Q that will be published on
		ReplyListenQ:       qr,                            // This is the lisened to by client for wakie-wakie on client side
		TickInMilliseconds: 100,                           // 100 milliseconds
		ReplyFx:            make(map[string]*ReplyFxType), //
	}
	return
}

func (ms *MsCfgType) SetEventPattern(pat string) {
	ms.EventPattern = pat
}

// setRedisPool - sets the redis poo info in the MsCfgType
//
// Example:
// 		micro := NewMsCfgType()
// 		micro.SetRedisPool(hdlr.gCfg.RedisPool)
func (ms *MsCfgType) SetRedisPool(pool *pool.Pool) {
	ms.RedisPool = pool
}

func (ms *MsCfgType) SetRedisConnectInfo(h, p, a string) {
	ms.RedisConnectHost = h
	ms.RedisConnectPort = p
	ms.RedisConnectAuth = a
}

// GetRedis gets the redis connection
func (ms *MsCfgType) GetRedis() (conn *redis.Client) {
	//if ms.conn != nil {
	//	conn = ms.conn
	//	return
	//}
	var err error
	conn, err = ms.RedisPool.Get()
	ms.Err = err
	//ms.conn = conn
	return
}

// PutRedis release the connection back to the connection pool
func (ms *MsCfgType) PutRedis(conn *redis.Client) {
	ms.RedisPool.Put(conn)
	// 	ms.conn = nil
}

func (ms *MsCfgType) SetupListen() {

	client, err := redis.Dial("tcp", ms.RedisConnectHost+":"+ms.RedisConnectPort)
	if err != nil {
		log.Fatal(err)
	}
	if ms.RedisConnectAuth != "" {
		err = client.Cmd("AUTH", ms.RedisConnectAuth).Err
		if err != nil {
			log.Fatal(err)
		} else {
			fmt.Fprintf(os.Stderr, "Success: Connected to redis-server with AUTH.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "Success: Connected to redis-server.\n")
	}

	ms.subClient = pubsub.NewSubClient(client) // subClient *pubsub.SubClient
	ms.subChan = make(chan *pubsub.SubResp)
	ms.timeout = make(chan bool, 1)

	mdata := make(map[string]string)
	mdata["ServerId"] = ms.ServerId
	fmt.Printf("%sListening for events that match: %s, %s\n", MiscLib.ColorCyan, ms.EventPattern, MiscLib.ColorReset)
	sr := ms.subClient.PSubscribe(ms.EventPattern)
	if sr.Err != nil {
		fmt.Fprintf(os.Stderr, "%sError: subscribe, %s.%s\n", MiscLib.ColorRed, sr.Err, MiscLib.ColorReset)
	}
}

// ms.TimeOutMessage(flag)
func (ms *MsCfgType) TimeOutMessage(f bool) {
	ms.DebugTimeoutMessage = f
}

func (ms *MsCfgType) ListenForServer(doWork WorkFuncType, wg *sync.WaitGroup) { // server *socketio.Server) {

	arb := make(map[string]interface{})
	arb["cmd"] = "at-top"
	doWork(arb)

	go func() {
		for {
			ms.subChan <- ms.subClient.Receive()
		}
	}()

	go func() {
		for {
			time.Sleep(time.Duration(ms.TickInMilliseconds) * time.Millisecond)
			ms.timeout <- true
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var sr *pubsub.SubResp
		counter := 0
		threshold := 100 // xyzzy - from config!!!
		for {
			select {
			case sr = <-ms.subChan:
				if db1 {
					fmt.Fprintf(os.Stderr, "%s**** Got a message, sr=%+v, is --->>>%s<<<---, AT:%s%s\n", MiscLib.ColorGreen, godebug.SVar(sr), sr.Message, godebug.LF(), MiscLib.ColorReset)
				}

				arb := make(map[string]interface{})
				arb["cmd"] = "expired"
				arb["val"] = string(sr.Message)
				doWork(arb)

			case <-ms.timeout: // the read from ms.subChan has timed out

				if ms.DebugTimeoutMessage {
					fmt.Fprintf(os.Stderr, "%s**** Got a timeout, AT:%s%s\n", MiscLib.ColorGreen, godebug.LF(), MiscLib.ColorReset)
				}

				// If stuck doing work - may need to kill/restart - server side timeout.
				counter++
				if counter > threshold {
					if repoll_db {
						fmt.Fprintf(os.Stderr, "%s**** timeout - results in a call to doWork(), AT:%s%s\n", MiscLib.ColorGreen, godebug.LF(), MiscLib.ColorReset)
					}
					arb := make(map[string]interface{})
					arb["cmd"] = "timeout-call"
					doWork(arb)
					counter = 0
				}

			}
		}
	}()

}

// SetReplyFunc needs to be called before sending a message if you want a call/responce type operation.
// If you want message send and forget then do not call this with a 'fx', leave ms.ReplyFunc nil.
func (ms *MsCfgType) SetReplyFunc(fx func(replyMessage *MsMessageToSend)) {
	ms.ReplyFunc = fx
}

func (ms *MsCfgType) Dump() {
	fmt.Printf("Dump out internal information on ms\n")
	fmt.Printf("ReplyFx = %s\n\n", godebug.SVarI(ms.ReplyFx))
}

func (hdlr *MsCfgType) ConnectToRedis() bool {
	// Note: best test for this is in the TabServer2 - test 0001 - checks that this works.
	var err error

	dflt := func(a string, d string) (rv string) {
		rv = a
		if rv == "" {
			rv = d
		}
		return
	}

	redis_host := dflt(hdlr.RedisConnectHost, "127.0.0.1")
	redis_port := dflt(hdlr.RedisConnectPort, "6379")
	redis_auth := hdlr.RedisConnectAuth

	if redis_auth == "" { // If Redis AUTH section
		hdlr.RedisPool, err = pool.New("tcp", redis_host+":"+redis_port, 20)
	} else {
		hdlr.RedisPool, err = pool.NewAuth("tcp", redis_host+":"+redis_port, 20, redis_auth)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: Failed to connect to redis-server.%s\n", MiscLib.ColorRed, MiscLib.ColorReset)
		fmt.Printf("Error: Failed to connect to redis-server.\n")
		// goftlmux.G_Log.Info("Error: Failed to connect to redis-server.\n")
		// logrus.Fatalf("Error: Failed to connect to redis-server.\n")
		return false
	} else {
		if db3 {
			fmt.Fprintf(os.Stderr, "%sSuccess: Connected to redis-server.%s\n", MiscLib.ColorGreen, MiscLib.ColorReset)
		}
	}

	return true
}

func (ms *MsCfgType) SetDbFlag(flag string, val bool) {
	switch flag {
	case "db1":
		db1 = val
	case "db2":
		db2 = val
	case "db3":
		db3 = val
	case "repoll_db":
		repoll_db = val
	default:
		fmt.Fprintf(os.Stderr, "*** Error - invalid debug flag %s, should be db1, db2, db3, repoll_db ***\n", flag)
	}
}

/*
-------------------------------------------------------
UUID Notes
-------------------------------------------------------

import (
	newUuid "github.com/pborman/uuid"
	"github.com/pschlump/Go-FTL/server/lib"
	"github.com/taskcluster/slugid-go/slugid"
)

	id0 := lib.GetUUIDAsString()
	id0_slug := UUIDToSlug(id0)
*/

func UUIDToSlug(uuid string) (slug string) {
	// slug = id
	uuidType := newUuid.Parse(uuid)
	if uuidType != nil {
		slug = slugid.Encode(uuidType)
		return
	}
	fmt.Fprintf(os.Stderr, "slug: ERROR: Cannot encode invalid uuid '%v' into a slug\n", uuid) // Xyzzy - logrus
	return
}

func SlugToUUID(slug string) (uuid string) {
	// uuid = slug
	uuidType := slugid.Decode(slug)
	if uuidType != nil {
		uuid = uuidType.String()
		return
	}
	fmt.Fprintf(os.Stderr, "slug: ERROR: Cannot decode invalid slug '%v' into a UUID\n", slug) // Xyzzy - logrus
	return
}

func StructToJson(mm interface{}) (rv string) {
	trv, err := json.Marshal(mm)
	if err != nil {
		rv = "{}"
		return
	}
	rv = string(trv)
	return
}

// --------------------------------------------------------------------------------------------------------------------------------

// ms.AddToFnMap(ms.GenCacheFn(id), url)
func (hdlr *MsCfgType) AddToFnMap(replace, fn, origFn, mt string) {

	fmt.Printf("AddToFnMap: replace=%s, fn=%s, %s\n", replace, fn, godebug.LF())

	conn := hdlr.GetRedis()
	defer hdlr.PutRedis(conn)

	// Replace			fn
	// From /ipfs/ID -> /t1/css/t1.css
	// From /ipfs/ID -> http://www.example.com/t1/css/t1.css
	type FCacheType struct {
		FileName       string `json:"FileName"`      // should change json to just "fn"
		State          string `json:"St,omitempty"`  //
		OrigFileName   string `json:"ofn,omitempty"` // if this has not been fetched yet, then 307 to this, Orig starts with http[s]?//...
		RemoteFileName string `json:"rfn,omitempty"` // if this has not been fetched yet, then 307 to this, Orig starts with http[s]?//...
		Replace        string `json:"rp,omitempty"`  //
		MimeType       string `json:"mt,omitempty"`  // The mime type of the file
	}

	key := "h2p3:" + replace
	// FCacheData := H2pppCommon.FCacheType{FileName: fn, Replace: replace, State: "200", OrigFileName: origFn, RemoteFileName: fn, MimeType: mt}
	FCacheData := FCacheType{FileName: fn, Replace: replace, State: "200", OrigFileName: origFn, RemoteFileName: fn, MimeType: mt}

	value := ""
	v, err := json.Marshal(FCacheData)
	if err != nil {
		value = "{}"
	} else {
		value = string(v)
	}

	err = conn.Cmd("SET", key, value).Err
	if err != nil {
		if db2 {
			fmt.Printf("Error on redis - file data not saved - get(%s): %s\n", key, err)
		}
		return
	}

	//	// fn						replace
	//	// From /t1/css/t1.css   -> /ipfs/ID
	//	// From http://www.example.com/t1/css/t1.css -> /ipfs/ID
	//	key1 := "h2p3:" + fn
	//	FCacheData := H2pppCommon.FCacheType{FileName: replace, State: "200", OrigFileName: fn, RemoteFileName: replace}
	//
	//	value1 := ""
	//	v, err := json.Marshal(FCacheData)
	//	if err != nil {
	//		value1 = "{}"
	//	} else {
	//		value1 = string(v)
	//	}
	//
	//	err = conn.Cmd("SET", key1, value1).Err
	//	if err != nil {
	//		if db2 {
	//			fmt.Printf("Error on redis - file data not saved - get(%s): %s\n", key1, err)
	//		}
	//		return
	//	}
	//
	//	fmt.Printf("Redis: SET %s %s, and... reverse %s %s\n", key, value, key1, value1)

	fmt.Printf("Redis: SET %s %s\n", key, value)
}

// ms.AddToFnMap(ms.GenCacheFn(id, path), url)
func (hdlr *MsCfgType) GenCacheFn(id, pth, urlPath string) string {
	return urlPath + "/" + id
}

func GetParam(Params []MsMessageParams, name string) string {
	for _, vv := range Params {
		if vv.Name == name {
			return vv.Value
		}
	}
	return ""
}

func GetParamIndex(Params []MsMessageParams, name string) int {
	for ii, vv := range Params {
		if vv.Name == name {
			return ii
		}
	}
	return -1
}

const ReturnPacked = true

func UUIDAsStr() (s_id string) {
	id, _ := uuid.NewV4()
	s_id = id.String()
	return
}

func UUIDAsStrPacked() (s_id string) {
	if ReturnPacked {
		s := UUIDAsStr()
		return UUIDToSlug(s)
	} else {
		return UUIDAsStr()
	}
}

var db1 = false
var db2 = false
var db3 = false

// var db12 = true
var repoll_db = false

/* vim: set noai ts=4 sw=4: */
