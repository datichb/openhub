package workflow

// PickRuntime returns the first preferred runtime the workflow allows
// (empty and unknown values skipped), else the workflow default. The
// preferences come from the most specific level first (project, then the
// user settings).
func (s *Spec) PickRuntime(prefs ...string) Runtime {
	for _, p := range prefs {
		if p != "" && s.AllowsRuntime(Runtime(p)) {
			return Runtime(p)
		}
	}
	return s.DefaultRuntime()
}
