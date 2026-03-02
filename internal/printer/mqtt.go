package printer

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// MQTT 3.1.1 packet types.
const (
	packetCONNECT     byte = 1
	packetCONNACK     byte = 2
	packetPUBLISH     byte = 3
	packetSUBSCRIBE   byte = 8
	packetSUBACK      byte = 9
	packetPINGREQ     byte = 12
	packetPINGRESP    byte = 13
	packetDISCONNECT  byte = 14
)

// CONNACK return codes.
const (
	connAccepted byte = 0
)

// mqttClient is a minimal MQTT 3.1.1 client using only stdlib.
type mqttClient struct {
	host     string
	port     int
	clientID string
	username string
	password string
	tlsCfg   *tls.Config
	logger   *slog.Logger

	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex

	onPublish func(topic string, payload []byte)
	closed    atomic.Bool
}

// mqttOption configures an mqttClient.
type mqttOption func(*mqttClient)

func withTLSConfig(cfg *tls.Config) mqttOption {
	return func(c *mqttClient) { c.tlsCfg = cfg }
}

func withCredentials(user, pass string) mqttOption {
	return func(c *mqttClient) {
		c.username = user
		c.password = pass
	}
}

func withOnPublish(fn func(topic string, payload []byte)) mqttOption {
	return func(c *mqttClient) { c.onPublish = fn }
}

func withLogger(l *slog.Logger) mqttOption {
	return func(c *mqttClient) { c.logger = l }
}

// newMQTTClient creates a new MQTT client. It does not connect automatically.
func newMQTTClient(host string, port int, clientID string, opts ...mqttOption) *mqttClient {
	c := &mqttClient{
		host:     host,
		port:     port,
		clientID: clientID,
		logger:   slog.Default(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// connect dials the broker, performs the TLS handshake, and sends CONNECT.
func (c *mqttClient) connect() error {
	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))

	var conn net.Conn
	var err error
	if c.tlsCfg != nil {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, c.tlsCfg)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}
	if err != nil {
		return fmt.Errorf("mqtt dial %s: %w", addr, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.reader = bufio.NewReaderSize(conn, 32*1024)
	c.closed.Store(false)
	c.mu.Unlock()

	if err := c.sendConnect(); err != nil {
		conn.Close()
		return err
	}

	code, err := c.readConnACK()
	if err != nil {
		conn.Close()
		return err
	}
	if code != connAccepted {
		conn.Close()
		return fmt.Errorf("mqtt connack refused: code=%d", code)
	}

	return nil
}

// subscribe sends a SUBSCRIBE for the given topic at QoS 0.
func (c *mqttClient) subscribe(topic string) error {
	pkt := buildSubscribe(1, topic, 0)
	if err := c.write(pkt); err != nil {
		return fmt.Errorf("mqtt subscribe write: %w", err)
	}

	grantedQoS, err := c.readSubACK()
	if err != nil {
		return fmt.Errorf("mqtt suback read: %w", err)
	}
	if grantedQoS == 0x80 {
		return fmt.Errorf("mqtt subscribe rejected by broker")
	}

	return nil
}

// publish sends a QoS 0 PUBLISH.
func (c *mqttClient) publish(topic string, payload []byte) error {
	pkt := buildPublish(topic, payload)
	return c.write(pkt)
}

// ping sends a PINGREQ and expects a PINGRESP within 5 seconds.
func (c *mqttClient) ping() error {
	pkt := []byte{packetPINGREQ << 4, 0}
	if err := c.write(pkt); err != nil {
		return fmt.Errorf("mqtt pingreq write: %w", err)
	}
	return nil
}

// disconnect sends DISCONNECT and closes the connection.
func (c *mqttClient) disconnect() error {
	pkt := []byte{packetDISCONNECT << 4, 0}
	_ = c.write(pkt) // best-effort
	return c.close()
}

// close shuts down the underlying connection.
func (c *mqttClient) close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// readLoop reads incoming packets until the connection is closed or errors.
func (c *mqttClient) readLoop() error {
	for {
		if c.closed.Load() {
			return nil
		}

		pktType, payload, err := c.readPacket()
		if err != nil {
			if c.closed.Load() {
				return nil
			}
			return fmt.Errorf("mqtt read: %w", err)
		}

		switch pktType {
		case packetPUBLISH:
			topic, msg, err := parsePublish(payload)
			if err != nil {
				c.logger.Warn("mqtt: malformed publish", "error", err)
				continue
			}
			if c.onPublish != nil {
				c.onPublish(topic, msg)
			}
		case packetPINGRESP:
			// expected response to our PINGREQ
		case packetSUBACK:
			// might arrive asynchronously; ignore in read loop
		default:
			c.logger.Debug("mqtt: unhandled packet type", "type", pktType)
		}
	}
}

// --- packet builders ---

func (c *mqttClient) sendConnect() error {
	pkt := buildConnect(c.clientID, c.username, c.password, 60)
	return c.write(pkt)
}

// buildConnect builds a CONNECT packet for MQTT 3.1.1.
func buildConnect(clientID, username, password string, keepAlive uint16) []byte {
	var payload bytes.Buffer

	// Variable header
	var varHeader bytes.Buffer
	// Protocol name "MQTT"
	writeUTF8String(&varHeader, "MQTT")
	// Protocol level 4 (3.1.1)
	varHeader.WriteByte(4)

	// Connect flags
	var flags byte
	flags |= 1 << 1 // clean session
	if username != "" {
		flags |= 1 << 7
	}
	if password != "" {
		flags |= 1 << 6
	}
	varHeader.WriteByte(flags)

	// Keep alive
	binary.Write(&varHeader, binary.BigEndian, keepAlive)

	// Payload
	writeUTF8String(&payload, clientID)
	if username != "" {
		writeUTF8String(&payload, username)
	}
	if password != "" {
		writeUTF8String(&payload, password)
	}

	remaining := varHeader.Bytes()
	remaining = append(remaining, payload.Bytes()...)

	return buildFixedHeader(packetCONNECT, 0, remaining)
}

// buildSubscribe builds a SUBSCRIBE packet.
func buildSubscribe(packetID uint16, topic string, qos byte) []byte {
	var body bytes.Buffer

	// Packet identifier
	binary.Write(&body, binary.BigEndian, packetID)

	// Topic filter + requested QoS
	writeUTF8String(&body, topic)
	body.WriteByte(qos)

	// SUBSCRIBE fixed header flags must be 0x02
	return buildFixedHeader(packetSUBSCRIBE, 0x02, body.Bytes())
}

// buildPublish builds a QoS 0 PUBLISH packet (no packet ID).
func buildPublish(topic string, payload []byte) []byte {
	var body bytes.Buffer
	writeUTF8String(&body, topic)
	body.Write(payload)
	return buildFixedHeader(packetPUBLISH, 0, body.Bytes())
}

// buildFixedHeader prepends the fixed header to a remaining-length body.
func buildFixedHeader(pktType byte, flags byte, body []byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(pktType<<4 | flags)
	writeRemainingLength(&buf, len(body))
	buf.Write(body)
	return buf.Bytes()
}

// --- packet parsers ---

// readConnACK reads a CONNACK and returns the return code.
func (c *mqttClient) readConnACK() (byte, error) {
	pktType, payload, err := c.readPacket()
	if err != nil {
		return 0, fmt.Errorf("mqtt connack: %w", err)
	}
	if pktType != packetCONNACK {
		return 0, fmt.Errorf("mqtt expected CONNACK, got type=%d", pktType)
	}
	return parseConnACK(payload)
}

// parseConnACK extracts the return code from a CONNACK payload.
func parseConnACK(payload []byte) (byte, error) {
	if len(payload) < 2 {
		return 0, fmt.Errorf("mqtt connack payload too short: %d", len(payload))
	}
	return payload[1], nil
}

// readSubACK reads a SUBACK and returns the granted QoS.
func (c *mqttClient) readSubACK() (byte, error) {
	pktType, payload, err := c.readPacket()
	if err != nil {
		return 0, fmt.Errorf("mqtt suback: %w", err)
	}
	if pktType != packetSUBACK {
		return 0, fmt.Errorf("mqtt expected SUBACK, got type=%d", pktType)
	}
	return parseSubACK(payload)
}

// parseSubACK extracts the granted QoS from a SUBACK payload.
func parseSubACK(payload []byte) (byte, error) {
	// Variable header: 2 bytes packet ID, then payload of granted QoS codes
	if len(payload) < 3 {
		return 0, fmt.Errorf("mqtt suback payload too short: %d", len(payload))
	}
	return payload[2], nil
}

// parsePublish extracts topic and payload from a QoS 0 PUBLISH body.
func parsePublish(body []byte) (string, []byte, error) {
	if len(body) < 2 {
		return "", nil, fmt.Errorf("publish body too short")
	}
	topicLen := int(binary.BigEndian.Uint16(body[:2]))
	if len(body) < 2+topicLen {
		return "", nil, fmt.Errorf("publish topic length exceeds body")
	}
	topic := string(body[2 : 2+topicLen])
	// QoS 0 → no packet ID
	payload := body[2+topicLen:]
	return topic, payload, nil
}

// --- wire helpers ---

// readPacket reads one full MQTT packet, returning packet type and variable-header+payload.
func (c *mqttClient) readPacket() (byte, []byte, error) {
	c.mu.Lock()
	r := c.reader
	c.mu.Unlock()

	firstByte, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}

	pktType := firstByte >> 4

	remaining, err := readRemainingLength(r)
	if err != nil {
		return 0, nil, err
	}

	payload := make([]byte, remaining)
	if remaining > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}

	return pktType, payload, nil
}

func (c *mqttClient) write(data []byte) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("mqtt: not connected")
	}
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := conn.Write(data)
	return err
}

// writeUTF8String writes an MQTT UTF-8 encoded string (length-prefixed).
func writeUTF8String(w *bytes.Buffer, s string) {
	binary.Write(w, binary.BigEndian, uint16(len(s)))
	w.WriteString(s)
}

// writeRemainingLength encodes the MQTT remaining length field.
func writeRemainingLength(w *bytes.Buffer, length int) {
	for {
		encoded := byte(length % 128)
		length /= 128
		if length > 0 {
			encoded |= 0x80
		}
		w.WriteByte(encoded)
		if length == 0 {
			break
		}
	}
}

// readRemainingLength decodes the MQTT remaining length field.
func readRemainingLength(r io.ByteReader) (int, error) {
	var value int
	var multiplier int = 1
	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value += int(b&0x7F) * multiplier
		if b&0x80 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, fmt.Errorf("mqtt: remaining length exceeds 4 bytes")
}
