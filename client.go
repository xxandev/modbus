package modbus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
)

type ApiClient interface {
	SetID(id byte)
	GetID() byte
	Encode(pdu *ProtocolDataUnit) (adu []byte, err error)
	Verify(aduRequest []byte, aduResponse []byte) (err error)
	Decode(adu []byte) (pdu *ProtocolDataUnit, err error)
}

type MBClient struct {
	mode string
	ApiClient
}

func NewClient() *MBClient {
	return &MBClient{}
}

func NewSClient(id byte, mode string) *MBClient {
	mbc := MBClient{}
	mbc.mode = mode
	switch strings.ToLower(mbc.mode) {
	case "rtu":
		mbc.ApiClient = newRtuClient()
	case "tcp":
		mbc.ApiClient = newTcpClient()
	case "ascii":
		mbc.ApiClient = newAsciiClient()
	default:
		mbc.ApiClient = newRtuClient()
	}
	mbc.SetID(id)
	return &mbc
}

// func (mbc *MBClient) GetID() byte     { return mbc.cln.GetID() }
func (mbc *MBClient) GetMode() string { return mbc.mode }

func (mbc *MBClient) Set(id byte, mode string) {
	mbc.mode = mode
	switch strings.ToLower(mode) {
	case "rtu":
		mbc.ApiClient = newRtuClient()
	case "tcp":
		mbc.ApiClient = newTcpClient()
	case "ascii":
		mbc.ApiClient = newAsciiClient()
	default:
		mbc.ApiClient = newRtuClient()
	}
	mbc.ApiClient.SetID(id)
}

func (c *MBClient) VerifyID(id byte) error {
	if id != c.ApiClient.GetID() {
		return fmt.Errorf("modbus: response slave id '%v' does not match request '%v'", id, c.ApiClient.GetID())
	}
	return nil
}

// Request:
//  Function code         : 1 byte (0x01)
//  Starting address      : 2 bytes
//  Quantity of coils     : 2 bytes
// Response:
//  Function code         : 1 byte (0x01)
//  Byte count            : 1 byte
//  Coil status           : N* bytes (=N or N+1)
func (c *MBClient) ReadCoils(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 2000 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 2000)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadCoils,
		Data:         dataBlock(address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x02)
//  Starting address      : 2 bytes
//  Quantity of inputs    : 2 bytes
// Response:
//  Function code         : 1 byte (0x02)
//  Byte count            : 1 byte
//  Input status          : N* bytes (=N or N+1)
func (c *MBClient) ReadDiscreteInputs(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 2000 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 2000)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadDiscreteInputs,
		Data:         dataBlock(address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x03)
//  Starting address      : 2 bytes
//  Quantity of registers : 2 bytes
// Response:
//  Function code         : 1 byte (0x03)
//  Byte count            : 1 byte
//  Register value        : Nx2 bytes
func (c *MBClient) ReadHoldingRegisters(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 125)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadHoldingRegisters,
		Data:         dataBlock(address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x04)
//  Starting address      : 2 bytes
//  Quantity of registers : 2 bytes
// Response:
//  Function code         : 1 byte (0x04)
//  Byte count            : 1 byte
//  Input registers       : N bytes
func (c *MBClient) ReadInputRegisters(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 125)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadInputRegisters,
		Data:         dataBlock(address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x05)
//  Output address        : 2 bytes
//  Output value          : 2 bytes
// Response:
//  Function code         : 1 byte (0x05)
//  Output address        : 2 bytes
//  Output value          : 2 bytes
func (c *MBClient) WriteSingleCoil(address, value uint16) ([]byte, error) {
	// The requested ON/OFF state can only be 0xFF00 and 0x0000
	if value != 0xFF00 && value != 0x0000 {
		return []byte{0x0}, fmt.Errorf("modbus: state '%v' must be either 0xFF00 (ON) or 0x0000 (OFF)", value)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteSingleCoil,
		Data:         dataBlock(address, value),
	})
}

func (c *MBClient) WriteSingleCoilBool(address uint16, value bool) ([]byte, error) {
	var v uint16 = 0x0000
	if value {
		v = 0xFF00
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteSingleCoil,
		Data:         dataBlock(address, v),
	})
}

// Request:
//  Function code         : 1 byte (0x06)
//  Register address      : 2 bytes
//  Register value        : 2 bytes
// Response:
//  Function code         : 1 byte (0x06)
//  Register address      : 2 bytes
//  Register value        : 2 bytes
func (c *MBClient) WriteSingleRegister(address, value uint16) ([]byte, error) {
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteSingleRegister,
		Data:         dataBlock(address, value),
	})
}

// Request:
//  Function code         : 1 byte (0x0F)
//  Starting address      : 2 bytes
//  Quantity of outputs   : 2 bytes
//  Byte count            : 1 byte
//  Outputs value         : N* bytes
// Response:
//  Function code         : 1 byte (0x0F)
//  Starting address      : 2 bytes
//  Quantity of outputs   : 2 bytes
func (c *MBClient) WriteMultipleCoils(address, quantity uint16, value []byte) ([]byte, error) {
	if quantity < 1 || quantity > 1968 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 1968)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteMultipleCoils,
		Data:         dataBlockSuffix(value, address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x10)
//  Starting address      : 2 bytes
//  Quantity of outputs   : 2 bytes
//  Byte count            : 1 byte
//  Registers value       : N* bytes
// Response:
//  Function code         : 1 byte (0x10)
//  Starting address      : 2 bytes
//  Quantity of registers : 2 bytes
func (c *MBClient) WriteMultipleRegisters(address, quantity uint16, value []byte) ([]byte, error) {
	if quantity < 1 || quantity > 123 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 123)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteMultipleRegisters,
		Data:         dataBlockSuffix(value, address, quantity),
	})
}

// Request:
//  Function code         : 1 byte (0x16)
//  Reference address     : 2 bytes
//  AND-mask              : 2 bytes
//  OR-mask               : 2 bytes
// Response:
//  Function code         : 1 byte (0x16)
//  Reference address     : 2 bytes
//  AND-mask              : 2 bytes
//  OR-mask               : 2 bytes
func (c *MBClient) MaskWriteRegister(address, andMask, orMask uint16) ([]byte, error) {
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeMaskWriteRegister,
		Data:         dataBlock(address, andMask, orMask),
	})
}

// Request:
//  Function code         : 1 byte (0x17)
//  Read starting address : 2 bytes
//  Quantity to read      : 2 bytes
//  Write starting address: 2 bytes
//  Quantity to write     : 2 bytes
//  Write byte count      : 1 byte
//  Write registers value : N* bytes
// Response:
//  Function code         : 1 byte (0x17)
//  Byte count            : 1 byte
//  Read registers value  : Nx2 bytes
func (c *MBClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) ([]byte, error) {
	if readQuantity < 1 || readQuantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity to read '%v' must be between '%v' and '%v',", readQuantity, 1, 125)
	}
	if writeQuantity < 1 || writeQuantity > 121 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity to write '%v' must be between '%v' and '%v',", writeQuantity, 1, 121)
	}
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadWriteMultipleRegisters,
		Data:         dataBlockSuffix(value, readAddress, readQuantity, writeAddress, writeQuantity),
	})
}

// Request:
//  Function code         : 1 byte (0x18)
//  FIFO pointer address  : 2 bytes
// Response:
//  Function code         : 1 byte (0x18)
//  Byte count            : 2 bytes
//  FIFO count            : 2 bytes
//  FIFO count            : 2 bytes (<=31)
//  FIFO value register   : Nx2 bytes
func (c *MBClient) ReadFIFOQueue(address uint16) ([]byte, error) {
	return c.ApiClient.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadFIFOQueue,
		Data:         dataBlock(address),
	})
}

type rtuClient struct {
	id byte
}

func newRtuClient() *rtuClient {
	return &rtuClient{}
}

func (rtu *rtuClient) SetID(id byte) {
	rtu.id = id
}

func (rtu *rtuClient) GetID() byte {
	return rtu.id
}

// Encode encodes PDU in a RTU frame:
//  Slave Address   : 1 byte
//  Function        : 1 byte
//  Data            : 0 up to 252 bytes
//  CRC             : 2 byte
func (rtu *rtuClient) Encode(pdu *ProtocolDataUnit) (adu []byte, err error) {
	length := len(pdu.Data) + 4
	if length > rtuMaxSize {
		err = fmt.Errorf("modbus: length of data '%v' must not be bigger than '%v'", length, rtuMaxSize)
		return
	}
	adu = make([]byte, length)

	adu[0] = rtu.id
	adu[1] = pdu.FunctionCode
	copy(adu[2:], pdu.Data)

	// Append crc
	var crc crc
	crc.reset().pushBytes(adu[0 : length-2])
	checksum := crc.value()

	adu[length-1] = byte(checksum >> 8)
	adu[length-2] = byte(checksum)
	return
}

// Verify verifies response length and slave id.
func (rtu *rtuClient) Verify(aduRequest []byte, aduResponse []byte) (err error) {
	length := len(aduResponse)
	// Minimum size (including address, function and CRC)
	if length < rtuMinSize {
		err = fmt.Errorf("modbus: response length '%v' does not meet minimum '%v'", length, rtuMinSize)
		return
	}
	// Slave address must match
	if aduResponse[0] != aduRequest[0] {
		err = fmt.Errorf("modbus: response slave id '%v' does not match request '%v'", aduResponse[0], aduRequest[0])
		return
	}
	return
}

// Decode extracts PDU from RTU frame and verify CRC.
func (rtu *rtuClient) Decode(adu []byte) (pdu *ProtocolDataUnit, err error) {
	length := len(adu)
	// Calculate checksum
	var crc crc
	crc.reset().pushBytes(adu[0 : length-2])
	checksum := uint16(adu[length-1])<<8 | uint16(adu[length-2])
	if checksum != crc.value() {
		err = fmt.Errorf("modbus: response crc '%v' does not match expected '%v'", checksum, crc.value())
		return
	}
	if rtu.id != adu[0] {
		err = fmt.Errorf("modbus invalid address %v != %v", rtu.id, adu[0])
	}
	// Function code & data
	pdu = &ProtocolDataUnit{}
	pdu.FunctionCode = adu[1]
	pdu.Data = adu[2 : length-2]
	return
}

type tcpClient struct {
	transactionId uint32
	id            byte
}

func newTcpClient() *tcpClient {
	return &tcpClient{}
}

func (tcp *tcpClient) SetID(id byte) {
	tcp.id = id
}

func (tcp *tcpClient) GetID() byte {
	return tcp.id
}

// Encode adds modbus application protocol header:
//  Transaction identifier: 2 bytes
//  Protocol identifier: 2 bytes
//  Length: 2 bytes
//  Unit identifier: 1 byte
//  Function code: 1 byte
//  Data: n bytes
func (tcp *tcpClient) Encode(pdu *ProtocolDataUnit) (adu []byte, err error) {
	adu = make([]byte, tcpHeaderSize+1+len(pdu.Data))

	// Transaction identifier
	transactionId := atomic.AddUint32(&tcp.transactionId, 1)
	binary.BigEndian.PutUint16(adu, uint16(transactionId))
	// Protocol identifier
	binary.BigEndian.PutUint16(adu[2:], tcpProtocolIdentifier)
	// Length = sizeof(SlaveId) + sizeof(FunctionCode) + Data
	length := uint16(1 + 1 + len(pdu.Data))
	binary.BigEndian.PutUint16(adu[4:], length)
	// Unit identifier
	adu[6] = tcp.id

	// PDU
	adu[tcpHeaderSize] = pdu.FunctionCode
	copy(adu[tcpHeaderSize+1:], pdu.Data)
	return
}

// Verify confirms transaction, protocol and unit id.
func (tcp *tcpClient) Verify(aduRequest []byte, aduResponse []byte) (err error) {
	// Transaction id
	responseVal := binary.BigEndian.Uint16(aduResponse)
	requestVal := binary.BigEndian.Uint16(aduRequest)
	if responseVal != requestVal {
		err = fmt.Errorf("modbus: response transaction id '%v' does not match request '%v'", responseVal, requestVal)
		return
	}
	// Protocol id
	responseVal = binary.BigEndian.Uint16(aduResponse[2:])
	requestVal = binary.BigEndian.Uint16(aduRequest[2:])
	if responseVal != requestVal {
		err = fmt.Errorf("modbus: response protocol id '%v' does not match request '%v'", responseVal, requestVal)
		return
	}
	// Unit id (1 byte)
	if aduResponse[6] != aduRequest[6] {
		err = fmt.Errorf("modbus: response unit id '%v' does not match request '%v'", aduResponse[6], aduRequest[6])
		return
	}
	return
}

// Decode extracts PDU from TCP frame:
//  Transaction identifier: 2 bytes
//  Protocol identifier: 2 bytes
//  Length: 2 bytes
//  Unit identifier: 1 byte
func (tcp *tcpClient) Decode(adu []byte) (pdu *ProtocolDataUnit, err error) {
	// Read length value in the header
	length := binary.BigEndian.Uint16(adu[4:])
	pduLength := len(adu) - tcpHeaderSize
	if pduLength <= 0 || pduLength != int(length-1) {
		err = fmt.Errorf("modbus: length in response '%v' does not match pdu data length '%v'", length-1, pduLength)
		return
	}
	if tcp.id != adu[6] {
		err = fmt.Errorf("modbus invalid address %v != %v", tcp.id, adu[0])
	}
	pdu = &ProtocolDataUnit{}
	// The first byte after header is function code
	pdu.FunctionCode = adu[tcpHeaderSize]
	pdu.Data = adu[tcpHeaderSize+1:]
	return
}

type asciiClient struct {
	id byte
}

func newAsciiClient() *asciiClient {
	return &asciiClient{}
}

func (ascii *asciiClient) SetID(id byte) {
	ascii.id = id
}

func (ascii *asciiClient) GetID() byte {
	return ascii.id
}

// Encode encodes PDU in a ASCII frame:
//  Start           : 1 char
//  Address         : 2 chars
//  Function        : 2 chars
//  Data            : 0 up to 2x252 chars
//  LRC             : 2 chars
//  End             : 2 chars
func (ascii *asciiClient) Encode(pdu *ProtocolDataUnit) (adu []byte, err error) {
	var buf bytes.Buffer

	if _, err = buf.WriteString(asciiStart); err != nil {
		return
	}
	if err = writeHex(&buf, []byte{ascii.id, pdu.FunctionCode}); err != nil {
		return
	}
	if err = writeHex(&buf, pdu.Data); err != nil {
		return
	}
	// Exclude the beginning colon and terminating CRLF pair characters
	var lrc lrc
	lrc.reset()
	lrc.pushByte(ascii.id).pushByte(pdu.FunctionCode).pushBytes(pdu.Data)
	if err = writeHex(&buf, []byte{lrc.value()}); err != nil {
		return
	}
	if _, err = buf.WriteString(asciiEnd); err != nil {
		return
	}
	adu = buf.Bytes()
	return
}

// Verify verifies response length, frame boundary and slave id.
func (ascii *asciiClient) Verify(aduRequest []byte, aduResponse []byte) (err error) {
	length := len(aduResponse)
	// Minimum size (including address, function and LRC)
	if length < asciiMinSize+6 {
		err = fmt.Errorf("modbus: response length '%v' does not meet minimum '%v'", length, 9)
		return
	}
	// Length excluding colon must be an even number
	if length%2 != 1 {
		err = fmt.Errorf("modbus: response length '%v' is not an even number", length-1)
		return
	}
	// First char must be a colon
	str := string(aduResponse[0:len(asciiStart)])
	if str != asciiStart {
		err = fmt.Errorf("modbus: response frame '%v'... is not started with '%v'", str, asciiStart)
		return
	}
	// 2 last chars must be \r\n
	str = string(aduResponse[len(aduResponse)-len(asciiEnd):])
	if str != asciiEnd {
		err = fmt.Errorf("modbus: response frame ...'%v' is not ended with '%v'", str, asciiEnd)
		return
	}
	// Slave id
	responseVal, err := readHex(aduResponse[1:])
	if err != nil {
		return
	}
	requestVal, err := readHex(aduRequest[1:])
	if err != nil {
		return
	}
	if responseVal != requestVal {
		err = fmt.Errorf("modbus: response slave id '%v' does not match request '%v'", responseVal, requestVal)
		return
	}
	return
}

// Decode extracts PDU from ASCII frame and verify LRC.
func (ascii *asciiClient) Decode(adu []byte) (pdu *ProtocolDataUnit, err error) {
	pdu = &ProtocolDataUnit{}
	// Slave address
	address, err := readHex(adu[1:])
	if err != nil {
		return
	}
	// Function code
	if pdu.FunctionCode, err = readHex(adu[3:]); err != nil {
		return
	}
	// Data
	dataEnd := len(adu) - 4
	data := adu[5:dataEnd]
	pdu.Data = make([]byte, hex.DecodedLen(len(data)))
	if _, err = hex.Decode(pdu.Data, data); err != nil {
		return
	}
	// LRC
	lrcVal, err := readHex(adu[dataEnd:])
	if err != nil {
		return
	}
	// Calculate checksum
	var lrc lrc
	lrc.reset()
	lrc.pushByte(address).pushByte(pdu.FunctionCode).pushBytes(pdu.Data)
	if lrcVal != lrc.value() {
		err = fmt.Errorf("modbus: response lrc '%v' does not match expected '%v'", lrcVal, lrc.value())
		return
	}
	return
}
