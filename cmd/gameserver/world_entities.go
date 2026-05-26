package main

import "sync/atomic"

const worldEntityIDStart int32 = 200000

var nextWorldEntityID int32 = worldEntityIDStart

func allocWorldEntityID() int32 {
	return atomic.AddInt32(&nextWorldEntityID, 1)
}
