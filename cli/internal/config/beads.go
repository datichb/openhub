package config

// BeadsConfig configures how oh treats Beads in the projects.
type BeadsConfig struct {
	// ZeroImpact is what oh does with the files `bd init` writes outside
	// .beads/ when a project is registered or Beads initialized: ask
	// (default: the changes are listed, then applied after a confirmation),
	// auto (applied, untracked settings files of other tools left alone) or
	// off. The commit of `bd init` is only undone with an explicit --yes.
	ZeroImpact string `mapstructure:"zero_impact" toml:"zero_impact,omitempty"`
}
