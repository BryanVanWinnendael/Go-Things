package install

// Info is what Windows knows about a suite app after it is installed: read
// from the per-user uninstall key the setup writes.
type Info struct {
	Version   string
	Location  string
	Uninstall string
	Icon      string
}
