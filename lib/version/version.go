package version

// VERSION identifies this build. The client reports it to the server on
// connect and the client list displays it, so a build stamp here is how an
// operator tells which nodes are still running an old binary. It is a var
// rather than a const so the build can stamp it:
//
//	-X ehang.io/nps/lib/version.VERSION=0.26.10+g1a2b3c4
var VERSION = "v0.27.13-hz1"

// Fork names the repository this binary was built from. Upstream has been
// unmaintained since 2021 and this tree carries fixes it never got, so every
// place a version is printed or reported says plainly which one it is.
const Fork = "hector918/nps"

// ForkMarker must appear in VERSION for every build from this fork, including
// the tag the release pipeline stamps in; build.release.sh refuses to build a
// tag without it.
const ForkMarker = "-hz"

// Protocol is the revision of the bridge protocol: the hello a client sends
// inside TLS, and what the mux and the control messages after it mean. The
// server turns away a client that reports a different one, so it changes
// whenever either side could no longer understand the other.
const Protocol = "2"
