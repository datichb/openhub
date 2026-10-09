package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// N4 (v5 critical, DC4): the infra agent reads freely, asks for secrets,
// never changes anything while `apply` is locked, and never prints
// credentials; global options before the sub-command do not help.
func TestInfraReadOnly(t *testing.T) {
	rules := effectiveRules(t, "infra", "infra")
	for _, cmd := range []string{
		"terraform plan", "terraform -chdir=infra validate", "terraform state list", "kubectl get pods -A",
		"kubectl describe deploy api", "kubectl logs api-1", "kubectl diff -f k8s/", "helm template ./chart", "helm list -A",
		"aws sts get-caller-identity", "aws ec2 describe-instances", "aws s3 ls", "gcloud compute instances list",
		"az group list", "pulumi preview", "vault status", "ansible-lint site.yml", "ansible-playbook site.yml --syntax-check",
	} {
		eff := ShellEffect(rules, cmd)
		if cmd == "terraform -chdir=infra validate" {
			assert.Equal(t, sessionspec.EffectAsk, eff, "a global option first: the agent's own rule (ask)")
			continue
		}
		assert.Equal(t, sessionspec.EffectAllow, eff, cmd)
	}
	for _, cmd := range []string{
		"kubectl get secret db -o yaml", "terraform output", "vault kv get secret/db", "helm get values api",
		"aws ssm get-parameter --name x", "kubectl port-forward svc/db 5432", "ansible-playbook site.yml --check",
	} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	mutations := []string{
		"terraform apply", "terraform -chdir=x apply -auto-approve", "terraform destroy", "tofu apply",
		"kubectl apply -f k8s/", "kubectl --context prod delete pod x", "kubectl exec -it api -- sh", "kubectl apply --dry-run=none -f x",
		"helm upgrade api ./chart", "helm uninstall api", "aws --profile p ec2 terminate-instances --instance-ids i-1",
		"aws s3 rm s3://b/x", "aws lambda invoke --function-name f out", "gcloud compute instances delete x", "az group delete -n x",
		"pulumi up", "vault kv put secret/x a=b", "vault login",
	}
	for _, cmd := range mutations {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), "asked to the agent: %s", cmd)
	}
	for _, cmd := range []string{
		"aws secretsmanager get-secret-value --secret-id x", "aws ssm get-parameter --name x --with-decryption",
		"aws ecr get-login-password", "aws sts assume-role --role-arn x", "gcloud auth print-access-token",
		"pulumi stack output --show-secrets", "kubectl config view --raw", "VAULT_TOKEN=x vault read x",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}

	// Session rules: refused while `apply` is locked, and in a workflow
	// that declares no checkpoint unlocking it; asked once it is unlocked.
	none := &sessionspec.WorkflowRuntime{ID: "libre", DefaultMode: "manuel"}
	gated := &sessionspec.WorkflowRuntime{ID: "infra-plan", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-apply", Behaviors: map[string]string{"manuel": "pause"}, Unlocks: []string{sessionspec.UnlockApply}},
	}}
	unlocked := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockApply: {Checkpoint: "cp-apply"}}}
	for _, cmd := range mutations {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(append(rules, SessionRules(none, "manuel", nil)...), cmd), "no apply checkpoint: %s", cmd)
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(append(rules, SessionRules(gated, "manuel", nil)...), cmd), "locked: %s", cmd)
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(append(rules, SessionRules(gated, "manuel", unlocked)...), cmd), "unlocked: %s", cmd)
	}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(append(rules, SessionRules(none, "manuel", nil)...), "kubectl get pods"))

	// The guard never widens an agent that does not run the tools.
	dev := effectiveRules(t, "developer", "developer")
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(dev, "kubectl get pods"))
	assert.Empty(t, InfraGuard(ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "git *": "allow"}})))
}

// N4 (DC3): the debugger reads and diagnoses; it asks before running the
// project's tests and cannot change files or run anything else.
func TestDebuggerShell(t *testing.T) {
	rules := effectiveRules(t, "debugger", "debugger")
	for _, cmd := range []string{"git log -5 --oneline", "git blame src/a.go", "rg 'panic' src", "tail -100 app.log", "bd show pt-1", "bd create 'Fix x'"} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"go test ./...", "npm test", "pytest -x tests/", ".venv/bin/python -m pytest"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"sed -i s/a/b/ x.go", "python3 -c 'print(1)'", "node -e 1", "curl https://x", "make", "rm x"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "src/a.go"))
}

// N4: the benchmarker no longer reads the environment and asks before the
// tools that run an arbitrary command.
func TestBenchmarkerShell(t *testing.T) {
	rules := effectiveRules(t, "benchmarker", "benchmarker")
	for _, cmd := range []string{"env", "env FOO=1 sh -c x", "printenv AWS_SECRET_ACCESS_KEY"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"hyperfine 'sh -c x'", "perf record -- ./x", "wrk -s x.lua http://localhost"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "go test -bench=. ./..."))
}
