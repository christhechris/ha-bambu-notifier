package printer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConnect(t *testing.T) {
	tests := []struct {
		name      string
		clientID  string
		username  string
		password  string
		keepAlive uint16
	}{
		{
			name:      "basic connection",
			clientID:  "test-client",
			username:  "bblp",
			password:  "12345678",
			keepAlive: 60,
		},
		{
			name:      "no credentials",
			clientID:  "anon-client",
			username:  "",
			password:  "",
			keepAlive: 30,
		},
		{
			name:      "username only",
			clientID:  "user-client",
			username:  "admin",
			password:  "",
			keepAlive: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := buildConnect(tt.clientID, tt.username, tt.password, tt.keepAlive)
			require.NotEmpty(t, pkt)

			// Verify fixed header: packet type is CONNECT
			assert.Equal(t, packetCONNECT, pkt[0]>>4, "packet type should be CONNECT")

			// Decode remaining length
			r := bufio.NewReader(bytes.NewReader(pkt[1:]))
			remaining, err := readRemainingLength(r)
			require.NoError(t, err)

			body := make([]byte, remaining)
			_, err = r.Read(body)
			require.NoError(t, err)

			// Variable header: protocol name "MQTT"
			assert.Equal(t, uint16(4), binary.BigEndian.Uint16(body[0:2]))
			assert.Equal(t, "MQTT", string(body[2:6]))

			// Protocol level 4
			assert.Equal(t, byte(4), body[6])

			// Connect flags
			flags := body[7]
			assert.True(t, flags&0x02 != 0, "clean session should be set")
			if tt.username != "" {
				assert.True(t, flags&0x80 != 0, "username flag should be set")
			} else {
				assert.True(t, flags&0x80 == 0, "username flag should not be set")
			}
			if tt.password != "" {
				assert.True(t, flags&0x40 != 0, "password flag should be set")
			} else {
				assert.True(t, flags&0x40 == 0, "password flag should not be set")
			}

			// Keep alive
			ka := binary.BigEndian.Uint16(body[8:10])
			assert.Equal(t, tt.keepAlive, ka)

			// Payload: client ID
			offset := 10
			clientIDLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
			offset += 2
			assert.Equal(t, tt.clientID, string(body[offset:offset+clientIDLen]))
			offset += clientIDLen

			if tt.username != "" {
				usernameLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
				offset += 2
				assert.Equal(t, tt.username, string(body[offset:offset+usernameLen]))
				offset += usernameLen
			}

			if tt.password != "" {
				passwordLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
				offset += 2
				assert.Equal(t, tt.password, string(body[offset:offset+passwordLen]))
			}
		})
	}
}

func TestBuildSubscribe(t *testing.T) {
	tests := []struct {
		name     string
		packetID uint16
		topic    string
		qos      byte
	}{
		{
			name:     "subscribe to report topic",
			packetID: 1,
			topic:    "device/SERIAL123/report",
			qos:      0,
		},
		{
			name:     "subscribe with different packet id",
			packetID: 42,
			topic:    "test/topic",
			qos:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := buildSubscribe(tt.packetID, tt.topic, tt.qos)
			require.NotEmpty(t, pkt)

			// Fixed header: SUBSCRIBE with reserved bits 0x02
			assert.Equal(t, packetSUBSCRIBE, pkt[0]>>4, "packet type should be SUBSCRIBE")
			assert.Equal(t, byte(0x02), pkt[0]&0x0F, "SUBSCRIBE flags should be 0x02")

			// Decode body
			r := bufio.NewReader(bytes.NewReader(pkt[1:]))
			remaining, err := readRemainingLength(r)
			require.NoError(t, err)

			body := make([]byte, remaining)
			_, err = r.Read(body)
			require.NoError(t, err)

			// Packet ID
			pid := binary.BigEndian.Uint16(body[0:2])
			assert.Equal(t, tt.packetID, pid)

			// Topic
			topicLen := int(binary.BigEndian.Uint16(body[2:4]))
			assert.Equal(t, tt.topic, string(body[4:4+topicLen]))

			// QoS
			assert.Equal(t, tt.qos, body[4+topicLen])
		})
	}
}

func TestBuildPublish(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		payload []byte
	}{
		{
			name:    "pushall command",
			topic:   "device/SERIAL123/request",
			payload: []byte(`{"pushing":{"sequence_id":"0","command":"pushall"}}`),
		},
		{
			name:    "empty payload",
			topic:   "test/topic",
			payload: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := buildPublish(tt.topic, tt.payload)
			require.NotEmpty(t, pkt)

			assert.Equal(t, packetPUBLISH, pkt[0]>>4, "packet type should be PUBLISH")

			// Parse the publish body
			r := bufio.NewReader(bytes.NewReader(pkt[1:]))
			remaining, err := readRemainingLength(r)
			require.NoError(t, err)

			body := make([]byte, remaining)
			_, err = r.Read(body)
			require.NoError(t, err)

			topic, msg, err := parsePublish(body)
			require.NoError(t, err)
			assert.Equal(t, tt.topic, topic)
			assert.Equal(t, tt.payload, msg)
		})
	}
}

func TestParsePublish(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		topic   string
		payload []byte
		wantErr bool
	}{
		{
			name:    "valid publish",
			body:    buildPublishBody("device/ABC/report", []byte(`{"print":{}}`)),
			topic:   "device/ABC/report",
			payload: []byte(`{"print":{}}`),
		},
		{
			name:    "empty payload",
			body:    buildPublishBody("test/topic", nil),
			topic:   "test/topic",
			payload: []byte{},
		},
		{
			name:    "body too short",
			body:    []byte{0},
			wantErr: true,
		},
		{
			name:    "topic length exceeds body",
			body:    []byte{0, 100, 'a'},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic, payload, err := parsePublish(tt.body)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.topic, topic)
			assert.Equal(t, tt.payload, payload)
		})
	}
}

func TestParseConnACK(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		code    byte
		wantErr bool
	}{
		{
			name:    "accepted",
			payload: []byte{0x00, connAccepted},
			code:    connAccepted,
		},
		{
			name:    "bad credentials",
			payload: []byte{0x00, connRefusedBadCredentials},
			code:    connRefusedBadCredentials,
		},
		{
			name:    "not authorized",
			payload: []byte{0x00, connRefusedNotAuthorized},
			code:    connRefusedNotAuthorized,
		},
		{
			name:    "payload too short",
			payload: []byte{0x00},
			wantErr: true,
		},
		{
			name:    "empty payload",
			payload: []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := ParseConnACK(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.code, code)
		})
	}
}

func TestParseSubACK(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		qos     byte
		wantErr bool
	}{
		{
			name:    "granted qos 0",
			payload: []byte{0x00, 0x01, 0x00},
			qos:     0x00,
		},
		{
			name:    "subscribe rejected",
			payload: []byte{0x00, 0x01, 0x80},
			qos:     0x80,
		},
		{
			name:    "payload too short",
			payload: []byte{0x00, 0x01},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qos, err := ParseSubACK(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.qos, qos)
		})
	}
}

func TestRemainingLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{name: "zero", length: 0},
		{name: "small", length: 42},
		{name: "127", length: 127},
		{name: "128", length: 128},
		{name: "16383", length: 16383},
		{name: "16384", length: 16384},
		{name: "large", length: 268435455},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeRemainingLength(&buf, tt.length)

			decoded, err := readRemainingLength(bufio.NewReader(&buf))
			require.NoError(t, err)
			assert.Equal(t, tt.length, decoded)
		})
	}
}

func TestReadRemainingLengthExceedsMax(t *testing.T) {
	// All continuation bits set → exceeds 4 bytes
	data := []byte{0x80, 0x80, 0x80, 0x80, 0x01}
	_, err := readRemainingLength(bufio.NewReader(bytes.NewReader(data)))
	assert.Error(t, err)
}

// buildPublishBody is a helper to construct a PUBLISH variable header + payload.
func buildPublishBody(topic string, payload []byte) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(len(topic)))
	buf.WriteString(topic)
	if payload != nil {
		buf.Write(payload)
	}
	return buf.Bytes()
}
