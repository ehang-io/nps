package nps_mux

// A ping's content is the sender's timestamp, which the peer echoes back so
// the sender can time the round trip. A client may put a fixed-length stats
// record in front of it: the server picks the record out of the ping it
// receives and echoes the whole content as before, and the client cuts the
// record off the echo. A peer that knows nothing of this just echoes, so old
// and new versions mix. The timestamp is text starting with a digit, which
// never equals the marker.
//
// A client does not send the record until the server has said it wants it. A
// server that takes stats puts StatsAccept in front of the pings it sends; an
// older server, or one of upstream's, never does, so a client connected to one
// keeps its host's data to itself.
const (
	// StatsAccept is the one byte a server that takes stats puts in front of
	// its ping.
	StatsAccept byte = 0xA6
	// StatsMagic is the first byte of a stats record.
	StatsMagic byte = 0xA5
	// StatsLen is the length of a stats record, marker included.
	StatsLen = 20
)

func hasStats(content []byte) bool {
	return len(content) >= StatsLen && content[0] == StatsMagic
}

func hasAccept(content []byte) bool {
	return len(content) > 0 && content[0] == StatsAccept
}

// stripStats returns content without what stats put in front of the
// timestamp.
func stripStats(content []byte) []byte {
	switch {
	case hasStats(content):
		return content[StatsLen:]
	case hasAccept(content):
		return content[1:]
	}
	return content
}
