package server

// Server holds the SQL server informations that are parsed from the header
type Server struct {
	Binary             string
	Port               int
	Socket             string
	Version            string
	VersionShort       string
	VersionDescription string
}
