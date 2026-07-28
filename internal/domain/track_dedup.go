package domain

// DeduplicateTrackNames appends _2, _3, ... suffixes to duplicate track names.
func DeduplicateTrackNames(tracks []MIDITrack) {
	nameCount := make(map[string]int)
	for i := range tracks {
		baseName := tracks[i].Name
		if baseName == "" {
			baseName = "Track"
		}
		count := nameCount[baseName]
		nameCount[baseName] = count + 1
		if count > 0 {
			tracks[i].Name = baseName + "_" + itoa(count+1)
		} else {
			tracks[i].Name = baseName
		}
	}
}

// itoa is a simple int to string conversion (avoid importing strconv for one function).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}