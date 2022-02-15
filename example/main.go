package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alpr777/modbus"
)

func init() {}

func main() {
	mb := modbus.NewTransporter()
	if err := mb.Connect("rtu", "/dev/ttyUSB0", 9600, 8, "N", 2, 1000, 30000); err != nil {
		log.Panic(err)
	}
	defer mb.Close()
	request := make(chan []byte, 999)
	defer close(request)

	go func() {
		devices := []*modbus.MBClient{
			modbus.NewSClient(0x12, "rtu"),
			modbus.NewSClient(0x13, "rtu"),
			modbus.NewSClient(0x14, "rtu"),
			modbus.NewSClient(0x15, "rtu"),
			modbus.NewSClient(0x16, "rtu"),
		}
		for n := range devices {
			req1, _ := devices[n].ReadCoils(0, 2)
			request <- req1
			time.Sleep(2000 * time.Millisecond)
			req2, _ := devices[n].ReadHoldingRegisters(0, 11)
			request <- req2
			time.Sleep(2000 * time.Millisecond)
		}
	}()

	go func() {
		timestamp := time.Now()
		for req := range request {
			timestamp = time.Now()
			id, fcode := mb.Spec(req)
			resp, warn, err := mb.Send(req)
			if err != nil {
				log.Panicf("error send %v[%v]: %v \n", id, fcode, err)
			}
			if warn != nil {
				log.Printf("warning send %v[%v]: %v \n", id, fcode, warn)
				continue
			}
			log.Printf("since %v, response %v[%v]: %v \n", time.Since(timestamp), id, fcode, resp)
		}
	}()

	ossigs := make(chan os.Signal, 1)
	signal.Notify(ossigs, os.Interrupt, os.Kill, syscall.SIGTERM)
	for range ossigs {
		mb.Close()
		os.Exit(0)
	}
}
