package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
)

// trackIDNamespace is a fixed namespace UUID used for generating
// deterministic track IDs from SHA-256 checksums.
var trackIDNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// ComputeTrackID generates a deterministic UUID based on track content.
// The same track content (name, channel, notes) always produces the same UUID.
func ComputeTrackID(name string, channel int, notes []MIDINote) uuid.UUID {
	h := sha256.New()

	// Include track name
	h.Write([]byte(name))

	// Include channel
	channelBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(channelBytes, uint32(channel))
	h.Write(channelBytes)

	// Include all notes in sorted order for determinism
	for _, n := range notes {
		// Pitch
		pitchBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(pitchBytes, uint32(n.Pitch))
		h.Write(pitchBytes)

		// Velocity
		velBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(velBytes, uint32(n.Velocity))
		h.Write(velBytes)

		// StartTime
		h.Write([]byte(fmt.Sprintf("%.10f", n.StartTime)))

		// EndTime
		h.Write([]byte(fmt.Sprintf("%.10f", n.EndTime)))
	}

	checksum := h.Sum(nil)

	// Use UUID v5 from the checksum to produce a deterministic UUID
	return uuid.NewHash(sha256.New(), trackIDNamespace, checksum, 5)
}