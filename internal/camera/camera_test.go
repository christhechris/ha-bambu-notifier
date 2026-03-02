package camera

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuthPacket(t *testing.T) {
	t.Parallel()

	pkt := buildAuthPacket("12345678")

	require.Len(t, pkt, authPacketSize)

	assert.Equal(t, uint32(authMagic), binary.LittleEndian.Uint32(pkt[0:4]))
	assert.Equal(t, uint32(authCommand), binary.LittleEndian.Uint32(pkt[4:8]))

	// Reserved bytes must be zero.
	assert.Equal(t, make([]byte, 8), pkt[8:16])

	// Username "bblp" null-padded to 32 bytes.
	assert.Equal(t, []byte("bblp"), pkt[16:20])
	assert.Equal(t, make([]byte, 28), pkt[20:48])

	// Access code null-padded to 32 bytes.
	assert.Equal(t, []byte("12345678"), pkt[48:56])
	assert.Equal(t, make([]byte, 24), pkt[56:80])
}

func TestBuildAuthPacket_longAccessCode(t *testing.T) {
	t.Parallel()

	longCode := "12345678901234567890123456789012extra"
	pkt := buildAuthPacket(longCode)

	require.Len(t, pkt, authPacketSize)

	// Only first 32 bytes should be written (copy truncates).
	assert.Equal(t,
		[]byte("12345678901234567890123456789012"),
		pkt[48:80],
	)
}

func TestParseFrameHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   []byte
		wantSize uint32
		wantErr  bool
	}{
		{
			name: "valid header",
			header: func() []byte {
				h := make([]byte, headerSize)
				binary.LittleEndian.PutUint32(h[:4], 1024)
				return h
			}(),
			wantSize: 1024,
		},
		{
			name:    "zero payload size",
			header:  make([]byte, headerSize),
			wantErr: true,
		},
		{
			name: "payload too large",
			header: func() []byte {
				h := make([]byte, headerSize)
				binary.LittleEndian.PutUint32(h[:4], 20<<20)
				return h
			}(),
			wantErr: true,
		},
		{
			name:    "header too short",
			header:  make([]byte, 4),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			size, err := parseFrameHeader(tt.header)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantSize, size)
			}
		})
	}
}

func TestValidateJPEG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid JPEG SOI",
			data:    []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00},
			wantErr: false,
		},
		{
			name:    "invalid SOI marker (PNG header)",
			data:    []byte{0x89, 0x50, 0x4E, 0x47},
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "too short",
			data:    []byte{0xFF},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateJPEG(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
