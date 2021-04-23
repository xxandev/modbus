package modbus

import (
	"fmt"
)

type RTUClient struct {
	rtuPackager
}

func NewRTUClient() *RTUClient {
	return &RTUClient{}
}

func (c *RTUClient) Set(slaveID byte) {
	c.SlaveId = slaveID
}

func (c *RTUClient) VerifyID(id byte) error {
	if id != c.SlaveId {
		return fmt.Errorf("modbus: response slave id '%v' does not match request '%v'", id, c.SlaveId)
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
func (c *RTUClient) ReadCoils(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 2000 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 2000)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) ReadDiscreteInputs(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 2000 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 2000)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) ReadHoldingRegisters(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 125)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) ReadInputRegisters(address, quantity uint16) ([]byte, error) {
	if quantity < 1 || quantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 125)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) WriteSingleCoil(address, value uint16) ([]byte, error) {
	// The requested ON/OFF state can only be 0xFF00 and 0x0000
	if value != 0xFF00 && value != 0x0000 {
		return []byte{0x0}, fmt.Errorf("modbus: state '%v' must be either 0xFF00 (ON) or 0x0000 (OFF)", value)
	}
	return c.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeWriteSingleCoil,
		Data:         dataBlock(address, value),
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
func (c *RTUClient) WriteSingleRegister(address, value uint16) ([]byte, error) {
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) WriteMultipleCoils(address, quantity uint16, value []byte) ([]byte, error) {
	if quantity < 1 || quantity > 1968 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 1968)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) WriteMultipleRegisters(address, quantity uint16, value []byte) ([]byte, error) {
	if quantity < 1 || quantity > 123 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity '%v' must be between '%v' and '%v',", quantity, 1, 123)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) MaskWriteRegister(address, andMask, orMask uint16) ([]byte, error) {
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) ([]byte, error) {
	if readQuantity < 1 || readQuantity > 125 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity to read '%v' must be between '%v' and '%v',", readQuantity, 1, 125)
	}
	if writeQuantity < 1 || writeQuantity > 121 {
		return []byte{0x0}, fmt.Errorf("modbus: quantity to write '%v' must be between '%v' and '%v',", writeQuantity, 1, 121)
	}
	return c.Encode(&ProtocolDataUnit{
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
func (c *RTUClient) ReadFIFOQueue(address uint16) ([]byte, error) {
	return c.Encode(&ProtocolDataUnit{
		FunctionCode: FuncCodeReadFIFOQueue,
		Data:         dataBlock(address),
	})
}
