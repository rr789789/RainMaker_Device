package local

import (
	"encoding/json"
	"fmt"
	"log"
)

// Protobuf message type constants (from esp_local_ctrl.proto)
const (
	TypeCmdGetPropertyCount   = 0
	TypeRespGetPropertyCount  = 1
	TypeCmdGetPropertyValues  = 4
	TypeRespGetPropertyValues = 5
	TypeCmdSetPropertyValues  = 6
	TypeRespSetPropertyValues = 7
)

const StatusSuccess = 0

type LocalCtrlMessage struct {
	Msg     uint32
	Payload []byte
}

// DecodeLocalCtrlMessage decodes a protobuf LocalCtrlMessage
func DecodeLocalCtrlMessage(data []byte) (*LocalCtrlMessage, error) {
	msg := &LocalCtrlMessage{}
	i := 0
	for i < len(data) {
		_, wireType, n := decodeTag(data[i:])
		i += n
		if wireType == 0 {
			_, n = decodeRawVarint(data[i:])
			i += n
		} else if wireType == 2 {
			rawLen, n := decodeRawVarint(data[i:])
			length := int(rawLen)
			i += n
			// We'll re-parse to get field numbers properly
			i += length
		}
	}

	// Re-parse properly
	i = 0
	for i < len(data) {
		fieldNum, wireType, n := decodeTag(data[i:])
		i += n
		if wireType == 0 {
			val, n2 := decodeRawVarint(data[i:])
			i += n2
			if fieldNum == 1 {
				msg.Msg = val
			}
		} else if wireType == 2 {
			rawLen, n2 := decodeRawVarint(data[i:])
			length := int(rawLen)
			i += n2
			if fieldNum == 1 {
				msg.Msg, _ = decodeRawVarint(data[i : i+length])
			} else if fieldNum >= 10 && fieldNum <= 15 {
				msg.Payload = data[i : i+length]
			}
			i += length
		}
	}
	return msg, nil
}

// EncodeLocalCtrlMessage encodes a LocalCtrlMessage to protobuf bytes
func EncodeLocalCtrlMessage(msg *LocalCtrlMessage) []byte {
	var buf []byte
	buf = append(buf, encodeTag(1, 0)...)
	buf = append(buf, encodeVarint(msg.Msg)...)
	if len(msg.Payload) > 0 {
		fieldNum := uint32(11)
		switch msg.Msg {
		case TypeRespGetPropertyCount:
			fieldNum = 11
		case TypeRespGetPropertyValues:
			fieldNum = 13
		case TypeRespSetPropertyValues:
			fieldNum = 15
		}
		buf = append(buf, encodeTag(fieldNum, 2)...)
		buf = append(buf, encodeVarint(uint32(len(msg.Payload)))...)
		buf = append(buf, msg.Payload...)
	}
	return buf
}

func BuildGetPropertyCountResponse() []byte {
	var payload []byte
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)
	payload = append(payload, encodeTag(2, 0)...)
	payload = append(payload, encodeVarint(2)...)
	msg := &LocalCtrlMessage{Msg: TypeRespGetPropertyCount, Payload: payload}
	return EncodeLocalCtrlMessage(msg)
}

func BuildGetPropertyValuesResponse(configJSON, paramsJSON []byte) []byte {
	var payload []byte
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)

	configProp := encodePropertyInfo(StatusSuccess, "config", 0, 0, configJSON)
	payload = append(payload, encodeTag(2, 2)...)
	payload = append(payload, encodeVarint(uint32(len(configProp)))...)
	payload = append(payload, configProp...)

	paramsProp := encodePropertyInfo(StatusSuccess, "params", 0, 0, paramsJSON)
	payload = append(payload, encodeTag(2, 2)...)
	payload = append(payload, encodeVarint(uint32(len(paramsProp)))...)
	payload = append(payload, paramsProp...)

	msg := &LocalCtrlMessage{Msg: TypeRespGetPropertyValues, Payload: payload}
	return EncodeLocalCtrlMessage(msg)
}

func encodePropertyInfo(status uint32, name string, typ uint32, flags uint32, value []byte) []byte {
	var buf []byte
	buf = append(buf, encodeTag(1, 0)...)
	buf = append(buf, encodeVarint(status)...)
	buf = append(buf, encodeTag(2, 2)...)
	buf = append(buf, encodeVarint(uint32(len(name)))...)
	buf = append(buf, []byte(name)...)
	buf = append(buf, encodeTag(3, 0)...)
	buf = append(buf, encodeVarint(typ)...)
	buf = append(buf, encodeTag(4, 0)...)
	buf = append(buf, encodeVarint(flags)...)
	buf = append(buf, encodeTag(5, 2)...)
	buf = append(buf, encodeVarint(uint32(len(value)))...)
	buf = append(buf, value...)
	return buf
}

func BuildSetPropertyValuesResponse() []byte {
	var payload []byte
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)
	msg := &LocalCtrlMessage{Msg: TypeRespSetPropertyValues, Payload: payload}
	return EncodeLocalCtrlMessage(msg)
}

func ParseSetPropertyValues(payload []byte) ([]byte, error) {
	i := 0
	for i < len(payload) {
		fieldNum, wireType, n := decodeTag(payload[i:])
		i += n
		if wireType == 2 {
			rawLen, n2 := decodeRawVarint(payload[i:])
			length := int(rawLen)
			i += n2
			if fieldNum == 1 {
				return parsePropertyValue(payload[i : i+length])
			}
			i += length
		} else if wireType == 0 {
			_, n2 := decodeRawVarint(payload[i:])
			i += n2
		}
	}
	return nil, fmt.Errorf("no property value found")
}

func parsePropertyValue(data []byte) ([]byte, error) {
	i := 0
	var valueBytes []byte
	for i < len(data) {
		fieldNum, wireType, n := decodeTag(data[i:])
		i += n
		if wireType == 2 {
			rawLen, n2 := decodeRawVarint(data[i:])
			length := int(rawLen)
			i += n2
			if fieldNum == 2 {
				valueBytes = data[i : i+length]
			}
			i += length
		} else if wireType == 0 {
			_, n2 := decodeRawVarint(data[i:])
			i += n2
		}
	}
	if valueBytes == nil {
		return nil, fmt.Errorf("no value bytes found")
	}
	return valueBytes, nil
}

// Protobuf varint helpers

// decodeTag decodes a protobuf tag into (fieldNumber, wireType, bytesConsumed)
func decodeTag(data []byte) (uint32, uint32, int) {
	val, n := decodeRawVarint(data)
	return val >> 3, val & 0x7, n
}

// decodeRawVarint decodes a raw varint, returns (value, bytesConsumed)
func decodeRawVarint(data []byte) (uint32, int) {
	var result uint32
	var shift uint
	n := 0
	for n < len(data) {
		b := data[n]
		result |= uint32(b&0x7F) << shift
		n++
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, n
}

func encodeTag(fieldNum, wireType uint32) []byte {
	return encodeVarint((fieldNum << 3) | wireType)
}

func encodeVarint(v uint32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v > 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func init() {
	_ = json.Marshal
	log.SetFlags(log.Ltime | log.Ldate)
}
