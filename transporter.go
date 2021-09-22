package modbus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/goburrow/serial"
)

type ApiTransporter interface {
	Connect() error
	Close() error
	Send([]byte) ([]byte, error, error)
}

type MBTransporter struct {
	mode        string
	addr        string
	baudrate    int
	databits    int
	parity      string
	stopbits    int
	timeout     int64
	idletimeout int64

	t ApiTransporter
}

func NewTransporter() *MBTransporter {
	return &MBTransporter{}
}

func (mbt *MBTransporter) GetMode() string       { return mbt.mode }
func (mbt *MBTransporter) GetAddress() string    { return mbt.addr }
func (mbt *MBTransporter) GetBaudRate() int      { return mbt.baudrate }
func (mbt *MBTransporter) GetDataBits() int      { return mbt.databits }
func (mbt *MBTransporter) GetParity() string     { return mbt.parity }
func (mbt *MBTransporter) GetStopBits() int      { return mbt.stopbits }
func (mbt *MBTransporter) GetTimeout() int64     { return mbt.timeout }
func (mbt *MBTransporter) GetIdleTimeout() int64 { return mbt.idletimeout }

func (mbt *MBTransporter) SetMode(value string)       { mbt.mode = value }
func (mbt *MBTransporter) SetAddress(value string)    { mbt.addr = value }
func (mbt *MBTransporter) SetBaudRate(value int)      { mbt.baudrate = value }
func (mbt *MBTransporter) SetDataBits(value int)      { mbt.databits = value }
func (mbt *MBTransporter) SetParity(value string)     { mbt.parity = value }
func (mbt *MBTransporter) SetStopBits(value int)      { mbt.stopbits = value }
func (mbt *MBTransporter) SetTimeout(value int64)     { mbt.timeout = value }
func (mbt *MBTransporter) SetIdleTimeout(value int64) { mbt.idletimeout = value }

func (mbt *MBTransporter) SetTCP(address string, timeout, idletimeout int64) {
	mbt.mode = "tcp"
	mbt.addr = address
	mbt.timeout = timeout
	mbt.idletimeout = idletimeout
}

func (mbt *MBTransporter) SetRTU(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	mbt.mode = "rtu"
	mbt.addr = address
	mbt.baudrate = baud
	mbt.databits = databits
	mbt.parity = parity
	mbt.stopbits = stopbits
	mbt.timeout = timeout
	mbt.idletimeout = idletimeout
}

func (mbt *MBTransporter) SetASCII(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	mbt.mode = "ascii"
	mbt.addr = address
	mbt.baudrate = baud
	mbt.databits = databits
	mbt.parity = parity
	mbt.stopbits = stopbits
	mbt.timeout = timeout
	mbt.idletimeout = idletimeout
}

func (mbt *MBTransporter) Connect() error {
	switch strings.ToLower(mbt.mode) {
	case "rtu":
		rtu := rtuTransporter{}
		rtu.Set(mbt.addr, mbt.baudrate, mbt.databits, mbt.parity, mbt.stopbits, mbt.timeout, mbt.idletimeout)
		if err := rtu.Connect(); err != nil {
			return err
		}
		mbt.t = &rtu
		return nil
	case "tcp":
		tcp := tcpTransporter{}
		tcp.Set(mbt.addr, mbt.timeout, mbt.idletimeout)
		if err := tcp.Connect(); err != nil {
			return err
		}
		mbt.t = &tcp
		return nil
	case "ascii":
		ascii := asciiTransporter{}
		ascii.Set(mbt.addr, mbt.baudrate, mbt.databits, mbt.parity, mbt.stopbits, mbt.timeout, mbt.idletimeout)
		if err := ascii.Connect(); err != nil {
			return err
		}
		mbt.t = &ascii
		return nil
	}
	return fmt.Errorf("unknown connection type")
}
func (mbt *MBTransporter) Send(aduRequest []byte) (aduResponse []byte, warn, err error) {
	return mbt.t.Send(aduRequest)
}

/*

























 */

// serialPort has configuration and I/O controller.
type serialPort struct {
	// Serial port configuration.
	serial.Config

	Logger      *log.Logger
	IdleTimeout time.Duration

	mu sync.Mutex
	// port is platform-dependent data structure for serial port.
	port         io.ReadWriteCloser
	lastActivity time.Time
	closeTimer   *time.Timer
}

func (mb *serialPort) Connect() (err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.connect()
}

// connect connects to the serial port if it is not connected. Caller must hold the mutex.
func (mb *serialPort) connect() error {
	if mb.port == nil {
		port, err := serial.Open(&mb.Config)
		if err != nil {
			return err
		}
		mb.port = port
	}
	return nil
}

func (mb *serialPort) Close() (err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.close()
}

// close closes the serial port if it is connected. Caller must hold the mutex.
func (mb *serialPort) close() (err error) {
	if mb.port != nil {
		err = mb.port.Close()
		mb.port = nil
	}
	return
}

func (mb *serialPort) logf(format string, v ...interface{}) {
	if mb.Logger != nil {
		mb.Logger.Printf(format, v...)
	}
}

func (mb *serialPort) startCloseTimer() {
	if mb.IdleTimeout <= 0 {
		return
	}
	if mb.closeTimer == nil {
		mb.closeTimer = time.AfterFunc(mb.IdleTimeout, mb.closeIdle)
	} else {
		mb.closeTimer.Reset(mb.IdleTimeout)
	}
}

// closeIdle closes the connection if last activity is passed behind IdleTimeout.
func (mb *serialPort) closeIdle() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.IdleTimeout <= 0 {
		return
	}
	idle := time.Now().Sub(mb.lastActivity)
	if idle >= mb.IdleTimeout {
		mb.logf("modbus: closing connection due to idle timeout: %v", idle)
		mb.close()
	}
}

// rtuSerialTransporter implements Transporter interface.
type rtuTransporter struct {
	serialPort
}

// func NewRTU(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) *RTUtransporter {
// 	return &RTUtransporter{}
// }

func (rtu *rtuTransporter) Set(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	rtu.Address = address
	rtu.BaudRate = baud
	rtu.DataBits = databits
	rtu.Parity = parity
	rtu.StopBits = stopbits
	rtu.Timeout = time.Duration(timeout) * time.Millisecond
	rtu.IdleTimeout = time.Duration(idletimeout) * time.Millisecond
}

func (rtu *rtuTransporter) Send(aduRequest []byte) (aduResponse []byte, warn, err error) {
	// Make sure port is connected
	if err = rtu.serialPort.connect(); err != nil {
		return
	}
	// Start the timer to close when idle
	rtu.serialPort.lastActivity = time.Now()
	rtu.serialPort.startCloseTimer()
	// Send the request
	rtu.serialPort.logf("modbus: sending % x\n", aduRequest)
	if _, err = rtu.port.Write(aduRequest); err != nil {
		return
	}
	function := aduRequest[1]
	functionFail := aduRequest[1] & 0x80
	bytesToRead := calculateResponseLength(aduRequest)
	time.Sleep(rtu.calculateDelay(len(aduRequest) + bytesToRead))

	var n int
	var n1 int
	var data [rtuMaxSize]byte
	//We first read the minimum length and then read either the full package
	//or the error package, depending on the error status (byte 2 of the response)
	n, warn = io.ReadAtLeast(rtu.port, data[:], rtuMinSize)
	if warn != nil {
		return
	}
	//if the function is correct
	if data[1] == function {
		//we read the rest of the bytes
		if n < bytesToRead {
			if bytesToRead > rtuMinSize && bytesToRead <= rtuMaxSize {
				if bytesToRead > n {
					n1, warn = io.ReadFull(rtu.port, data[n:bytesToRead])
					n += n1
				}
			}
		}
	} else if data[1] == functionFail {
		//for error we need to read 5 bytes
		if n < rtuExceptionSize {
			n1, warn = io.ReadFull(rtu.port, data[n:rtuExceptionSize])
		}
		n += n1
	}
	if warn != nil {
		return
	}
	aduResponse = data[:n]
	rtu.serialPort.logf("modbus: received % x\n", aduResponse)
	return
}

// calculateDelay roughly calculates time needed for the next frame.
// See MODBUS over Serial Line - Specification and Implementation Guide (page 13).
func (mb *rtuTransporter) calculateDelay(chars int) time.Duration {
	var characterDelay, frameDelay int // us

	if mb.BaudRate <= 0 || mb.BaudRate > 19200 {
		characterDelay = 750
		frameDelay = 1750
	} else {
		characterDelay = 15000000 / mb.BaudRate
		frameDelay = 35000000 / mb.BaudRate
	}
	return time.Duration(characterDelay*chars+frameDelay) * time.Microsecond
}

func calculateResponseLength(adu []byte) int {
	length := rtuMinSize
	switch adu[1] {
	case FuncCodeReadDiscreteInputs,
		FuncCodeReadCoils:
		count := int(binary.BigEndian.Uint16(adu[4:]))
		length += 1 + count/8
		if count%8 != 0 {
			length++
		}
	case FuncCodeReadInputRegisters,
		FuncCodeReadHoldingRegisters,
		FuncCodeReadWriteMultipleRegisters:
		count := int(binary.BigEndian.Uint16(adu[4:]))
		length += 1 + count*2
	case FuncCodeWriteSingleCoil,
		FuncCodeWriteMultipleCoils,
		FuncCodeWriteSingleRegister,
		FuncCodeWriteMultipleRegisters:
		length += 4
	case FuncCodeMaskWriteRegister:
		length += 6
	case FuncCodeReadFIFOQueue:
		// undetermined
	default:
	}
	return length
}

/*


















 */

// tcpTransporter implements Transporter interface.
type tcpTransporter struct {
	// Connect string
	Address string
	// Connect & Read timeout
	Timeout time.Duration
	// Idle timeout to close the connection
	IdleTimeout time.Duration
	// Transmission logger
	Logger *log.Logger

	// TCP connection
	mu           sync.Mutex
	conn         net.Conn
	closeTimer   *time.Timer
	lastActivity time.Time
}

// func NewTCP(address string, timeout, idletimeout int64) *TCPtransporter {
// 	// t := &TCPtransporter{}
// 	// t.Address = address
// 	// t.Timeout = tcpTimeout
// 	// t.IdleTimeout = tcpIdleTimeout
// 	return &TCPtransporter{}
// }

func (tcp *tcpTransporter) Set(address string, timeout, idletimeout int64) {
	tcp.Address = address
	tcp.Timeout = time.Duration(timeout) * time.Millisecond
	tcp.IdleTimeout = time.Duration(idletimeout) * time.Millisecond
}

// Send sends data to server and ensures response length is greater than header length.
func (tcp *tcpTransporter) Send(aduRequest []byte) (aduResponse []byte, warn, err error) {
	tcp.mu.Lock()
	defer tcp.mu.Unlock()

	// Establish a new connection if not connected
	if err = tcp.connect(); err != nil {
		return
	}
	// Set timer to close when idle
	tcp.lastActivity = time.Now()
	tcp.startCloseTimer()
	// Set write and read timeout
	var timeout time.Time
	if tcp.Timeout > 0 {
		timeout = tcp.lastActivity.Add(tcp.Timeout)
	}
	if err = tcp.conn.SetDeadline(timeout); err != nil {
		return
	}
	// Send data
	tcp.logf("modbus: sending % x", aduRequest)
	if _, err = tcp.conn.Write(aduRequest); err != nil {
		return
	}
	// Read header first
	var data [tcpMaxLength]byte
	if _, err = io.ReadFull(tcp.conn, data[:tcpHeaderSize]); err != nil {
		return
	}
	// fmt.Println("===============", data)
	// Read length, ignore transaction & protocol id (4 bytes)
	length := int(binary.BigEndian.Uint16(data[4:]))
	if length <= 0 {
		tcp.flush(data[:])
		err = fmt.Errorf("modbus: length in response header '%v' must not be zero", length)
		return
	}
	if length > (tcpMaxLength - (tcpHeaderSize - 1)) {
		tcp.flush(data[:])
		err = fmt.Errorf("modbus: length in response header '%v' must not greater than '%v'", length, tcpMaxLength-tcpHeaderSize+1)
		return
	}
	// Skip unit id
	length += tcpHeaderSize - 1
	if _, err = io.ReadFull(tcp.conn, data[tcpHeaderSize:length]); err != nil {
		return
	}
	aduResponse = data[:length]
	tcp.logf("modbus: received % x\n", aduResponse)
	return
}

// Connect establishes a new connection to the address in Address.
// Connect and Close are exported so that multiple requests can be done with one session
func (tcp *tcpTransporter) Connect() error {
	tcp.mu.Lock()
	defer tcp.mu.Unlock()

	return tcp.connect()
}

func (tcp *tcpTransporter) connect() error {
	if tcp.conn == nil {
		dialer := net.Dialer{Timeout: tcp.Timeout}
		conn, err := dialer.Dial("tcp", tcp.Address)
		if err != nil {
			return err
		}
		tcp.conn = conn
	}
	return nil
}

func (tcp *tcpTransporter) startCloseTimer() {
	if tcp.IdleTimeout <= 0 {
		return
	}
	if tcp.closeTimer == nil {
		tcp.closeTimer = time.AfterFunc(tcp.IdleTimeout, tcp.closeIdle)
	} else {
		tcp.closeTimer.Reset(tcp.IdleTimeout)
	}
}

// Close closes current connection.
func (tcp *tcpTransporter) Close() error {
	tcp.mu.Lock()
	defer tcp.mu.Unlock()

	return tcp.close()
}

// flush flushes pending data in the connection,
// returns io.EOF if connection is closed.
func (tcp *tcpTransporter) flush(b []byte) (err error) {
	if err = tcp.conn.SetReadDeadline(time.Now()); err != nil {
		return
	}
	// Timeout setting will be reset when reading
	if _, err = tcp.conn.Read(b); err != nil {
		// Ignore timeout error
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			err = nil
		}
	}
	return
}

func (tcp *tcpTransporter) logf(format string, v ...interface{}) {
	if tcp.Logger != nil {
		tcp.Logger.Printf(format, v...)
	}
}

// closeLocked closes current connection. Caller must hold the mutex before calling this method.
func (tcp *tcpTransporter) close() (err error) {
	if tcp.conn != nil {
		err = tcp.conn.Close()
		tcp.conn = nil
	}
	return
}

// closeIdle closes the connection if last activity is passed behind IdleTimeout.
func (tcp *tcpTransporter) closeIdle() {
	tcp.mu.Lock()
	defer tcp.mu.Unlock()

	if tcp.IdleTimeout <= 0 {
		return
	}
	idle := time.Now().Sub(tcp.lastActivity)
	if idle >= tcp.IdleTimeout {
		tcp.logf("modbus: closing connection due to idle timeout: %v", idle)
		tcp.close()
	}
}

/*











 */

// asciiSerialTransporter implements Transporter interface.
type asciiTransporter struct {
	serialPort
}

func (ascii *asciiTransporter) Set(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	ascii.Address = address
	ascii.BaudRate = baud
	ascii.DataBits = databits
	ascii.Parity = parity
	ascii.StopBits = stopbits
	ascii.Timeout = time.Duration(timeout) * time.Millisecond
	ascii.IdleTimeout = time.Duration(idletimeout) * time.Millisecond
}

func (ascii *asciiTransporter) Send(aduRequest []byte) (aduResponse []byte, warn, err error) {
	ascii.serialPort.mu.Lock()
	defer ascii.serialPort.mu.Unlock()

	// Make sure port is connected
	if err = ascii.serialPort.connect(); err != nil {
		return
	}
	// Start the timer to close when idle
	ascii.serialPort.lastActivity = time.Now()
	ascii.serialPort.startCloseTimer()

	// Send the request
	ascii.serialPort.logf("modbus: sending %q\n", aduRequest)
	if _, err = ascii.port.Write(aduRequest); err != nil {
		return
	}
	// Get the response
	var n int
	var data [asciiMaxSize]byte
	length := 0
	for {
		if n, err = ascii.port.Read(data[length:]); err != nil {
			return
		}
		length += n
		if length >= asciiMaxSize || n == 0 {
			break
		}
		// Expect end of frame in the data received
		if length > asciiMinSize {
			if string(data[length-len(asciiEnd):length]) == asciiEnd {
				break
			}
		}
	}
	aduResponse = data[:length]
	ascii.serialPort.logf("modbus: received %q\n", aduResponse)
	return
}

// writeHex encodes byte to string in hexadecimal, e.g. 0xA5 => "A5"
// (encoding/hex only supports lowercase string).
func writeHex(buf *bytes.Buffer, value []byte) (err error) {
	var str [2]byte
	for _, v := range value {
		str[0] = asciiHexTable[v>>4]
		str[1] = asciiHexTable[v&0x0F]

		if _, err = buf.Write(str[:]); err != nil {
			return
		}
	}
	return
}

// readHex decodes hexa string to byte, e.g. "8C" => 0x8C.
func readHex(data []byte) (value byte, err error) {
	var dst [1]byte
	if _, err = hex.Decode(dst[:], data[0:2]); err != nil {
		return
	}
	value = dst[0]
	return
}
