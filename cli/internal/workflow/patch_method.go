package workflow

// checkpointMethod merges `method:` of a checkpoint patch (R7): a method
// drops the checkpoint from the sessions of another method, so giving one
// to an inherited checkpoint, or changing it, is a loosening. A checkpoint
// added by the patch may carry one (it only adds a step).
func (p *patcher) checkpointMethod(base string, old *CheckpointSpec, v CheckpointSpec) {
	if !p.has(base+".method") || v.Method == old.Method {
		return
	}
	p.loosening(base+".method", methodText(v.Method), methodText(old.Method))
}

func methodText(m Method) string {
	if m == "" {
		return "none"
	}
	return string(m)
}
