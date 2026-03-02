package camera

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	authPacketSize = 80
	authMagic      = 0x40
	authCommand    = 0x3000
	headerSize     = 16
	usernameField  = "bblp"
	fieldLen       = 32
	dialTimeout    = 10 * time.Second
	maxPayloadSize = 10 << 20 // 10 MB sanity limit
)

// CaptureFrame connects to a Bambu printer's camera service over TLS
// and returns a single JPEG frame. The camera protocol runs on a
// dedicated TCP port (typically 6000) using JPEG-over-TLS.
func CaptureFrame(host string, port int, accessCode string, tlsInsecure bool) ([]byte, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: tlsInsecure, //nolint:gosec // Bambu printers use self-signed certificates
	})
	if err != nil {
		return nil, fmt.Errorf("camera dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, writeErr := conn.Write(buildAuthPacket(accessCode)); writeErr != nil {
		return nil, fmt.Errorf("camera auth write: %w", writeErr)
	}

	var hdr [headerSize]byte
	if _, readErr := io.ReadFull(conn, hdr[:]); readErr != nil {
		return nil, fmt.Errorf("camera read header: %w", readErr)
	}

	payloadSize, err := parseFrameHeader(hdr[:])
	if err != nil {
		return nil, err
	}

	jpeg := make([]byte, payloadSize)
	if _, readErr := io.ReadFull(conn, jpeg); readErr != nil {
		return nil, fmt.Errorf("camera read payload: %w", readErr)
	}

	if valErr := validateJPEG(jpeg); valErr != nil {
		return nil, valErr
	}

	return jpeg, nil
}

// buildAuthPacket constructs the 80-byte authentication packet for
// the Bambu camera protocol.
//
// Layout (all fields little-endian):
//
//	Offset  0: uint32 payload size marker (0x40 = 64)
//	Offset  4: uint32 command type (0x3000)
//	Offset  8: 8 bytes reserved (zero)
//	Offset 16: 32 bytes null-padded username ("bblp")
//	Offset 48: 32 bytes null-padded access code
func buildAuthPacket(accessCode string) []byte {
	pkt := make([]byte, authPacketSize)

	binary.LittleEndian.PutUint32(pkt[0:4], authMagic)
	binary.LittleEndian.PutUint32(pkt[4:8], authCommand)

	copy(pkt[16:16+fieldLen], usernameField)
	copy(pkt[48:48+fieldLen], accessCode)

	return pkt
}

// parseFrameHeader extracts the JPEG payload size from the 16-byte
// frame header.
func parseFrameHeader(hdr []byte) (uint32, error) {
	if len(hdr) < headerSize {
		return 0, fmt.Errorf("camera: header too short (%d bytes)", len(hdr))
	}

	size := binary.LittleEndian.Uint32(hdr[:4])
	if size == 0 || size > maxPayloadSize {
		return 0, fmt.Errorf("camera: invalid payload size %d", size)
	}

	return size, nil
}

// validateJPEG checks that data begins with the JPEG SOI marker.
func validateJPEG(data []byte) error {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return fmt.Errorf("camera: invalid JPEG (missing SOI marker)")
	}
	return nil
}
