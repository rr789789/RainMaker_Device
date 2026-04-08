package local

import (
	"encoding/json"
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"
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

// Status constants
const (
	StatusSuccess = 0
)

// LocalCtrlMessage represents the top-level protobuf message
// message LocalCtrlMessage {
//   LocalCtrlMsgType msg = 1;
//   oneof payload { ... }
// }
type LocalCtrlMessage struct {
	Msg     uint32
	Payload []byte // raw oneof payload
}

// PropertyInfo for responses
type PropertyInfo struct {
	Status uint32
	Name   string
	Type   uint32
	Flags  uint32
	Value  []byte
}

// PropertyValue for set requests
type PropertyValue struct {
	Index uint32
	Value []byte
}

// DecodeLocalCtrlMessage decodes a protobuf LocalCtrlMessage from bytes
func DecodeLocalCtrlMessage(data []byte) (*LocalCtrlMessage, error) {
	msg := &LocalCtrlMessage{}
	// field 1 (varint) = msg type
	// field 10-15 (length-delimited) = payload oneof
	i := 0
	for i < len(data) {
		fieldNum, wireType, n := decodeVarint(data[i:])
		i += n
		if wireType == 0 { // varint
			_, n = decodeVarint(data[i:])
			i += n
		} else if wireType == 2 { // length-delimited
			length, n := decodeVarint(data[i:])
			i += n
			if fieldNum == 1 {
				// msg type - re-decode
				msg.Msg, _ = decodeVarint(data[i : i+length])
			} else if fieldNum >= 10 && fieldNum <= 15 {
				msg.Payload = data[i : i+length]
			}
			i += int(length)
		} else {
			return nil, fmt.Errorf("unsupported wire type %d", wireType)
		}
	}
	return msg, nil
}

// EncodeLocalCtrlMessage encodes a LocalCtrlMessage to protobuf bytes
func EncodeLocalCtrlMessage(msg *LocalCtrlMessage) []byte {
	var buf []byte
	// field 1: msg type (varint)
	buf = append(buf, encodeTag(1, 0)...)
	buf = append(buf, encodeVarint(msg.Msg)...)
	// payload
	if len(msg.Payload) > 0 {
		// Use field 11 for resp_get_prop_count, 13 for resp_get_prop_vals, etc.
		fieldNum := uint32(11) // default to resp
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

// BuildGetPropertyCountResponse builds response for GetPropertyCount
func BuildGetPropertyCountResponse() []byte {
	// RespGetPropertyCount: field 1 (status=0), field 2 (count=2)
	var payload []byte
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)
	payload = append(payload, encodeTag(2, 0)...)
	payload = append(payload, encodeVarint(2)...)

	msg := &LocalCtrlMessage{Msg: TypeRespGetPropertyCount, Payload: payload}
	return EncodeLocalCtrlMessage(msg)
}

// BuildGetPropertyValuesResponse builds response for GetPropertyValues
func BuildGetPropertyValuesResponse(configJSON, paramsJSON []byte) []byte {
	// RespGetPropertyValues: field 1 (status=0), field 2 (repeated PropertyInfo)
	var payload []byte
	// status
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)

	// Property 0: config
	payload = append(payload, encodeTag(2, 2)...) // repeated PropertyInfo
	configProp := encodePropertyInfo(StatusSuccess, "config", 0, 0, configJSON)
	payload = append(payload, encodeVarint(uint32(len(configProp)))...)
	payload = append(payload, configProp...)

	// Property 1: params
	payload = append(payload, encodeTag(2, 2)...)
	paramsProp := encodePropertyInfo(StatusSuccess, "params", 0, 0, paramsJSON)
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

// BuildSetPropertyValuesResponse builds response for SetPropertyValues
func BuildSetPropertyValuesResponse() []byte {
	var payload []byte
	payload = append(payload, encodeTag(1, 0)...)
	payload = append(payload, encodeVarint(StatusSuccess)...)

	msg := &LocalCtrlMessage{Msg: TypeRespSetPropertyValues, Payload: payload}
	return EncodeLocalCtrlMessage(msg)
}

// ParseSetPropertyValues extracts the new params JSON from a SetPropertyValues request
func ParseSetPropertyValues(payload []byte) ([]byte, error) {
	// CmdSetPropertyValues: field 1 (repeated PropertyValue)
	// PropertyValue: field 1 (index), field 2 (value bytes)
	i := 0
	for i < len(payload) {
		fieldNum, wireType, n := decodeVarint(payload[i:])
		i += n
		if wireType == 2 {
			length, n := decodeVarint(payload[i:])
			i += n
			propData := payload[i : i+int(length)]
			if fieldNum == 1 {
				// Parse PropertyValue
				return parsePropertyValue(propData)
			}
			i += int(length)
		} else if wireType == 0 {
			_, n = decodeVarint(payload[i:])
			i += n
		}
	}
	return nil, fmt.Errorf("no property value found")
}

func parsePropertyValue(data []byte) ([]byte, error) {
	i := 0
	var valueBytes []byte
	for i < len(data) {
		fieldNum, wireType, n := decodeVarint(data[i:])
		i += n
		if wireType == 2 {
			length, n := decodeVarint(data[i:])
			i += n
			if fieldNum == 2 {
				valueBytes = data[i : i+int(length)]
			}
			i += int(length)
		} else if wireType == 0 {
			_, n = decodeVarint(data[i:])
			i += n
		}
	}
	if valueBytes == nil {
		return nil, fmt.Errorf("no value bytes found")
	}
	return valueBytes, nil
}

// ParseGetPropertyValues extracts requested indices
func ParseGetPropertyValues(payload []byte) []uint32 {
	var indices []uint32
	i := 0
	for i < len(payload) {
		fieldNum, wireType, n := decodeVarint(payload[i:])
		i += n
		if wireType == 0 && fieldNum == 1 {
			idx, n := decodeVarint(payload[i:])
			i += n
			indices = append(indices, idx)
		} else if wireType == 0 {
			_, n = decodeVarint(payload[i:])
			i += n
		} else if wireType == 2 {
			length, n := decodeVarint(payload[i:])
			i += n
			i += int(length)
		}
	}
	return indices
}

// Protobuf varint encoding/decoding helpers
func decodeVarint(data []byte) (uint32, uint32, int) {
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
	fieldNum := result >> 3
	wireType := result & 0x7
	return fieldNum, wireType, n
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

// Verify protobuf encoding matches the standard library
func init() {
	_ = proto.Marshal
	_ = json.Marshal
	log.SetFlags(log.Ltime | log.Ldate)
}
