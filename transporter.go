package modbus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/goburrow/serial"
)

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
type RTUtransporter struct {
	serialPort
}

func NewRTU() *RTUtransporter {
	return &RTUtransporter{}
}
func (t *RTUtransporter) Set(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	t.Address = address
	t.BaudRate = baud
	t.DataBits = databits
	t.Parity = parity
	t.StopBits = stopbits
	t.Timeout = time.Duration(timeout) * time.Millisecond
	t.IdleTimeout = time.Duration(idletimeout) * time.Millisecond
}

func (mb *RTUtransporter) Send(aduRequest []byte) (aduResponse []byte, warn, err error) {
	// Make sure port is connected
	if err = mb.serialPort.connect(); err != nil {
		return
	}
	// Start the timer to close when idle
	mb.serialPort.lastActivity = time.Now()
	mb.serialPort.startCloseTimer()
	// Send the request
	mb.serialPort.logf("modbus: sending % x\n", aduRequest)
	if _, err = mb.port.Write(aduRequest); err != nil {
		return
	}
	function := aduRequest[1]
	functionFail := aduRequest[1] & 0x80
	bytesToRead := calculateResponseLength(aduRequest)
	time.Sleep(mb.calculateDelay(len(aduRequest) + bytesToRead))

	var n int
	var n1 int
	var data [rtuMaxSize]byte
	//We first read the minimum length and then read either the full package
	//or the error package, depending on the error status (byte 2 of the response)
	n, warn = io.ReadAtLeast(mb.port, data[:], rtuMinSize)
	if warn != nil {
		return
	}
	//if the function is correct
	if data[1] == function {
		//we read the rest of the bytes
		if n < bytesToRead {
			if bytesToRead > rtuMinSize && bytesToRead <= rtuMaxSize {
				if bytesToRead > n {
					n1, warn = io.ReadFull(mb.port, data[n:bytesToRead])
					n += n1
				}
			}
		}
	} else if data[1] == functionFail {
		//for error we need to read 5 bytes
		if n < rtuExceptionSize {
			n1, warn = io.ReadFull(mb.port, data[n:rtuExceptionSize])
		}
		n += n1
	}
	if warn != nil {
		return
	}
	aduResponse = data[:n]
	mb.serialPort.logf("modbus: received % x\n", aduResponse)
	return
}

// calculateDelay roughly calculates time needed for the next frame.
// See MODBUS over Serial Line - Specification and Implementation Guide (page 13).
func (mb *RTUtransporter) calculateDelay(chars int) time.Duration {
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
type TCPtransporter struct {
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

func NewTCP(address string) *TCPtransporter {
	h := &TCPtransporter{}
	h.Address = address
	h.Timeout = tcpTimeout
	h.IdleTimeout = tcpIdleTimeout
	return h
}

// Send sends data to server and ensures response length is greater than header length.
func (mb *TCPtransporter) Send(aduRequest []byte) (aduResponse []byte, err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Establish a new connection if not connected
	if err = mb.connect(); err != nil {
		return
	}
	// Set timer to close when idle
	mb.lastActivity = time.Now()
	mb.startCloseTimer()
	// Set write and read timeout
	var timeout time.Time
	if mb.Timeout > 0 {
		timeout = mb.lastActivity.Add(mb.Timeout)
	}
	if err = mb.conn.SetDeadline(timeout); err != nil {
		return
	}
	// Send data
	mb.logf("modbus: sending % x", aduRequest)
	if _, err = mb.conn.Write(aduRequest); err != nil {
		return
	}
	// Read header first
	var data [tcpMaxLength]byte
	if _, err = io.ReadFull(mb.conn, data[:tcpHeaderSize]); err != nil {
		return
	}
	// fmt.Println("===============", data)
	// Read length, ignore transaction & protocol id (4 bytes)
	length := int(binary.BigEndian.Uint16(data[4:]))
	if length <= 0 {
		mb.flush(data[:])
		err = fmt.Errorf("modbus: length in response header '%v' must not be zero", length)
		return
	}
	if length > (tcpMaxLength - (tcpHeaderSize - 1)) {
		mb.flush(data[:])
		err = fmt.Errorf("modbus: length in response header '%v' must not greater than '%v'", length, tcpMaxLength-tcpHeaderSize+1)
		return
	}
	// Skip unit id
	length += tcpHeaderSize - 1
	if _, err = io.ReadFull(mb.conn, data[tcpHeaderSize:length]); err != nil {
		return
	}
	aduResponse = data[:length]
	mb.logf("modbus: received % x\n", aduResponse)
	return
}

// Connect establishes a new connection to the address in Address.
// Connect and Close are exported so that multiple requests can be done with one session
func (mb *TCPtransporter) Connect() error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.connect()
}

func (mb *TCPtransporter) connect() error {
	if mb.conn == nil {
		dialer := net.Dialer{Timeout: mb.Timeout}
		conn, err := dialer.Dial("tcp", mb.Address)
		if err != nil {
			return err
		}
		mb.conn = conn
	}
	return nil
}

func (mb *TCPtransporter) startCloseTimer() {
	if mb.IdleTimeout <= 0 {
		return
	}
	if mb.closeTimer == nil {
		mb.closeTimer = time.AfterFunc(mb.IdleTimeout, mb.closeIdle)
	} else {
		mb.closeTimer.Reset(mb.IdleTimeout)
	}
}

// Close closes current connection.
func (mb *TCPtransporter) Close() error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	return mb.close()
}

// flush flushes pending data in the connection,
// returns io.EOF if connection is closed.
func (mb *TCPtransporter) flush(b []byte) (err error) {
	if err = mb.conn.SetReadDeadline(time.Now()); err != nil {
		return
	}
	// Timeout setting will be reset when reading
	if _, err = mb.conn.Read(b); err != nil {
		// Ignore timeout error
		if netError, ok := err.(net.Error); ok && netError.Timeout() {
			err = nil
		}
	}
	return
}

func (mb *TCPtransporter) logf(format string, v ...interface{}) {
	if mb.Logger != nil {
		mb.Logger.Printf(format, v...)
	}
}

// closeLocked closes current connection. Caller must hold the mutex before calling this method.
func (mb *TCPtransporter) close() (err error) {
	if mb.conn != nil {
		err = mb.conn.Close()
		mb.conn = nil
	}
	return
}

// closeIdle closes the connection if last activity is passed behind IdleTimeout.
func (mb *TCPtransporter) closeIdle() {
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

/*











 */

// asciiSerialTransporter implements Transporter interface.
type ASCIItransporter struct {
	serialPort
}

func (t *ASCIItransporter) Set(address string, baud, databits int, parity string, stopbits int, timeout, idletimeout int64) {
	t.Address = address
	t.BaudRate = baud
	t.DataBits = databits
	t.Parity = parity
	t.StopBits = stopbits
	t.Timeout = time.Duration(timeout) * time.Millisecond
	t.IdleTimeout = time.Duration(idletimeout) * time.Millisecond
}

func (mb *ASCIItransporter) Send(aduRequest []byte) (aduResponse []byte, err error) {
	mb.serialPort.mu.Lock()
	defer mb.serialPort.mu.Unlock()

	// Make sure port is connected
	if err = mb.serialPort.connect(); err != nil {
		return
	}
	// Start the timer to close when idle
	mb.serialPort.lastActivity = time.Now()
	mb.serialPort.startCloseTimer()

	// Send the request
	mb.serialPort.logf("modbus: sending %q\n", aduRequest)
	if _, err = mb.port.Write(aduRequest); err != nil {
		return
	}
	// Get the response
	var n int
	var data [asciiMaxSize]byte
	length := 0
	for {
		if n, err = mb.port.Read(data[length:]); err != nil {
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
	mb.serialPort.logf("modbus: received %q\n", aduResponse)
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
